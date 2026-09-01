from __future__ import annotations

import os
import re
from dataclasses import dataclass
from typing import TYPE_CHECKING, Mapping

if TYPE_CHECKING:
    from .v1 import agent_runtime_pb2


# ---------------------------------------------------------------------------
# ModelConfig - one-call complete model config with resolved credentials
# Not serialized, not persisted, not in audit objects
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ModelConfig:
    """Complete model configuration produced by ModelConfigResolver.

    Short-lived object, used only within one model call:
    - not written to logs
    - not part of event stream
    - not in checkpoint
    """

    profile_id: str
    source: str
    protocol: str
    base_url: str
    model_name: str
    runtime_scope: str
    api_key: str

    @classmethod
    def from_protobuf(cls, connection: "agent_runtime_pb2.ModelConnection") -> "ModelConfig":
        """Create a ModelConfig from a protobuf ModelConnection.

        This is the single construction point for the legacy single-agent
        runtime path. The service layer uses this to lift credentials out
        of the transport layer before executors touch them.
        """
        return cls(
            profile_id=connection.profile_id,
            source=connection.source,
            protocol=connection.protocol,
            base_url=connection.base_url,
            model_name=connection.model_name,
            runtime_scope="agent",
            api_key=connection.api_key,
        )


# ---------------------------------------------------------------------------
# CredentialRef validation
# ---------------------------------------------------------------------------

_CREDENTIAL_REF_RE = re.compile(r"^(environment|profile):(.+)$")


def validate_credential_ref(ref: str) -> tuple[str, str]:
    """Validate credential_ref format, return (kind, value).

    Raises:
        ValueError: invalid format.
    """
    match = _CREDENTIAL_REF_RE.match(ref)
    if not match:
        raise ValueError(
            f"credential_ref must be 'environment:<scope>' or 'profile:<id>', got {ref!r}"
        )
    return match.group(1), match.group(2)


# ---------------------------------------------------------------------------
# Error types
# ---------------------------------------------------------------------------


class CredentialNotFoundError(RuntimeError):
    """credential_ref could not be resolved."""

    def __init__(self, ref: str, detail: str = "") -> None:
        msg = f"credential_ref {ref!r} could not be resolved"
        if detail:
            msg = f"{msg}: {detail}"
        super().__init__(msg)
        self.ref = ref


class ModelConfigPreparationError(RuntimeError):
    """Preparation-stage model config resolution failure. No engine fallback."""


class CredentialAccessDeniedError(ModelConfigPreparationError):
    """Credential was found but access was denied (authentication failure)."""


class CredentialProviderUnavailableError(ModelConfigPreparationError):
    """Credential provider is temporarily unavailable (retryable)."""


# ---------------------------------------------------------------------------
# CredentialResolver - resolve credential references
# Phase 1 supports: environment:go and environment:deepagent
# profile:<id> fails without a configured adapter
# ---------------------------------------------------------------------------

# Go runtime environment credentials (Go Control Plane side)
_ENV_GO_BASE_URL = "LLM_BASE_URL"
_ENV_GO_API_KEY = "LLM_API_KEY"
_ENV_GO_MODEL = "LLM_MODEL"

# DeepAgent runtime environment credentials (Python Agent Runtime side)
_ENV_DA_BASE_URL = "MODEL_BASE_URL"
_ENV_DA_API_KEY = "MODEL_API_KEY"
_ENV_DA_MODEL = "MODEL_NAME"


class CredentialResolver:
    """Resolve credential_ref to actual credentials.

    Phase 1 supports:
    - environment:go        -> LLM_BASE_URL + LLM_API_KEY
    - environment:deepagent -> MODEL_BASE_URL + MODEL_API_KEY
    - profile:<id>          -> requires production Secret Provider adapter, currently fails
    """

    def __init__(self, env: Mapping[str, str] | None = None) -> None:
        self._env = os.environ if env is None else dict(env)

    def resolve(
        self,
        ref: str,
        *,
        model_name: str = "",
    ) -> tuple[str, str, str]:
        """Resolve credential reference to (base_url, api_key, resolved_model_name).

        Raises:
            CredentialNotFoundError: reference cannot be resolved.
        """
        kind, value = validate_credential_ref(ref)

        if kind == "environment":
            return self._resolve_environment(value, model_name=model_name)

        # profile:<id> - Phase 1 has no Secret Provider
        raise CredentialNotFoundError(
            ref,
            detail="profile-based credential resolution requires a production CredentialResolver adapter; "
            "use 'environment:go' or 'environment:deepagent' for the first phase",
        )

    def _resolve_environment(
        self,
        scope: str,
        *,
        model_name: str = "",
    ) -> tuple[str, str, str]:
        if scope == "go":
            base_url = self._env.get(_ENV_GO_BASE_URL, "")
            api_key = self._env.get(_ENV_GO_API_KEY, "")
            resolved_model = model_name or self._env.get(_ENV_GO_MODEL, "")
            return base_url, api_key, resolved_model

        if scope == "deepagent":
            base_url = self._env.get(_ENV_DA_BASE_URL, "")
            api_key = self._env.get(_ENV_DA_API_KEY, "")
            resolved_model = model_name or self._env.get(_ENV_DA_MODEL, "")
            return base_url, api_key, resolved_model

        raise CredentialNotFoundError(
            f"environment:{scope}",
            detail=f"unknown environment scope {scope!r}; supported: 'go', 'deepagent'",
        )


# ---------------------------------------------------------------------------
# ModelConfigResolver - combine ModelSelection metadata with credentials
# ---------------------------------------------------------------------------


class ModelConfigResolver:
    """Combine ModelSelection with credentials into a complete ModelConfig."""

    def __init__(self, credential_resolver: CredentialResolver | None = None) -> None:
        self._credential_resolver = credential_resolver or CredentialResolver()

    def resolve(
        self,
        *,
        profile_id: str,
        source: str,
        protocol: str,
        model_name: str,
        runtime_scope: str,
        credential_ref: str,
    ) -> ModelConfig:
        """Resolve to a complete ModelConfig.

        Raises:
            ModelConfigPreparationError: preparation-stage resolution failure.
        """
        if not profile_id or not source or not protocol:
            raise ModelConfigPreparationError(
                "Model selection is incomplete: profile_id, source, and protocol are required"
            )
        if not credential_ref:
            raise ModelConfigPreparationError(
                "credential_ref is required for model resolution"
            )

        try:
            base_url, api_key, resolved_model_name = self._credential_resolver.resolve(
                credential_ref,
                model_name=model_name,
            )
        except CredentialNotFoundError as exc:
            raise ModelConfigPreparationError(str(exc)) from exc

        if not api_key:
            raise ModelConfigPreparationError(
                f"API key not found for credential_ref {credential_ref!r}"
            )

        final_model_name = resolved_model_name or model_name
        if not final_model_name:
            raise ModelConfigPreparationError(
                "model_name could not be resolved from selection or environment"
            )

        return ModelConfig(
            profile_id=profile_id,
            source=source,
            protocol=protocol,
            base_url=base_url,
            model_name=final_model_name,
            runtime_scope=runtime_scope,
            api_key=api_key,
        )
