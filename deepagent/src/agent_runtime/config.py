from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Mapping


MIB = 1024 * 1024


class RuntimeConfigError(ValueError):
    """Raised when the runtime would start with an unsafe or invalid config."""


@dataclass(frozen=True)
class RuntimeSettings:
    host: str = "127.0.0.1"
    port: int = 50051
    insecure: bool = False
    tls_cert_file: Path | None = None
    tls_key_file: Path | None = None
    tls_client_ca_file: Path | None = None
    max_concurrency: int = 4
    deepagent_concurrency: int = 1
    max_pending: int = 16
    max_request_bytes: int = 8 * MIB
    max_event_bytes: int = 4 * MIB
    max_artifact_bytes: int = 2 * MIB
    max_output_bytes: int = MIB
    event_buffer_size: int = 16
    shutdown_grace_seconds: float = 10.0
    work_dir: Path = Path("runs/runtime")
    enable_fake_executor: bool = False
    collaboration_enabled: bool = True
    collaboration_engine_allowlist: tuple[str, ...] = ("native",)
    collaboration_autogen_enabled: bool = False
    collaboration_max_concurrency: int = 4
    collaboration_max_pending: int = 16
    collaboration_checkpoint_max_bytes: int = MIB
    collaboration_default_engine: str = "native"
    collaboration_default_trigger_mode: str = "mention_only"

    @property
    def bind_address(self) -> str:
        return f"{self.host}:{self.port}"

    @property
    def server_options(self) -> tuple[tuple[str, int], ...]:
        return (
            ("grpc.max_receive_message_length", self.max_request_bytes),
            ("grpc.max_send_message_length", self.max_event_bytes),
        )

    def validate(self) -> None:
        if not self.host.strip():
            raise RuntimeConfigError("AGENT_RUNTIME_HOST must not be empty")
        if not 0 <= self.port <= 65535:
            raise RuntimeConfigError("AGENT_RUNTIME_PORT must be between 0 and 65535")
        for name, value in (
            ("AGENT_RUNTIME_MAX_CONCURRENCY", self.max_concurrency),
            ("AGENT_RUNTIME_DEEPAGENT_CONCURRENCY", self.deepagent_concurrency),
            ("AGENT_RUNTIME_MAX_REQUEST_BYTES", self.max_request_bytes),
            ("AGENT_RUNTIME_MAX_EVENT_BYTES", self.max_event_bytes),
            ("AGENT_RUNTIME_MAX_ARTIFACT_BYTES", self.max_artifact_bytes),
            ("AGENT_RUNTIME_MAX_OUTPUT_BYTES", self.max_output_bytes),
            ("AGENT_RUNTIME_EVENT_BUFFER_SIZE", self.event_buffer_size),
        ):
            if value <= 0:
                raise RuntimeConfigError(f"{name} must be positive")
        if self.deepagent_concurrency > self.max_concurrency:
            raise RuntimeConfigError("DeepAgent concurrency cannot exceed total concurrency")
        if self.max_pending < 0:
            raise RuntimeConfigError("AGENT_RUNTIME_MAX_PENDING must not be negative")
        if self.shutdown_grace_seconds < 0:
            raise RuntimeConfigError("AGENT_RUNTIME_SHUTDOWN_GRACE_SECONDS must not be negative")
        if not self.collaboration_engine_allowlist:
            raise RuntimeConfigError("COLLABORATION_ENGINE_ALLOWLIST must contain at least one engine")
        if self.collaboration_default_engine not in self.collaboration_engine_allowlist:
            raise RuntimeConfigError("COLLABORATION_DEFAULT_ENGINE must be in the engine allowlist")
        if self.collaboration_default_trigger_mode not in {"mention_only", "automatic"}:
            raise RuntimeConfigError("COLLABORATION_DEFAULT_TRIGGER_MODE must be mention_only or automatic")
        if self.collaboration_autogen_enabled and "autogen" not in self.collaboration_engine_allowlist:
            raise RuntimeConfigError("COLLABORATION_AUTOGEN_ENABLED requires autogen in the engine allowlist")
        if self.collaboration_max_concurrency <= 0:
            raise RuntimeConfigError("COLLABORATION_MAX_CONCURRENCY must be positive")
        if self.collaboration_max_pending < 0:
            raise RuntimeConfigError("COLLABORATION_MAX_PENDING must not be negative")
        if self.collaboration_checkpoint_max_bytes <= 0:
            raise RuntimeConfigError("COLLABORATION_CHECKPOINT_MAX_BYTES must be positive")
        if self.insecure:
            if not _is_loopback(self.host) and self.host not in {"0.0.0.0", "::"}:
                raise RuntimeConfigError("insecure runtime must bind loopback or an explicit container interface")
            return
        if self.tls_cert_file is None or self.tls_key_file is None:
            raise RuntimeConfigError(
                "TLS certificate and key are required unless AGENT_RUNTIME_INSECURE=true"
            )
        for path in (self.tls_cert_file, self.tls_key_file, self.tls_client_ca_file):
            if path is not None and not path.is_file():
                raise RuntimeConfigError(f"TLS file does not exist: {path}")
            if path is not None:
                try:
                    path.read_bytes()
                except OSError as exc:
                    raise RuntimeConfigError(f"TLS file is not readable: {path}") from exc

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> "RuntimeSettings":
        values = os.environ if env is None else env
        settings = cls(
            host=_string(values.get("AGENT_RUNTIME_HOST")) or "127.0.0.1",
            port=_int(values.get("AGENT_RUNTIME_PORT"), 50051),
            insecure=_bool(values.get("AGENT_RUNTIME_INSECURE"), False),
            tls_cert_file=_path(values.get("AGENT_RUNTIME_TLS_CERT_FILE")),
            tls_key_file=_path(values.get("AGENT_RUNTIME_TLS_KEY_FILE")),
            tls_client_ca_file=_path(values.get("AGENT_RUNTIME_TLS_CLIENT_CA_FILE")),
            max_concurrency=_int(values.get("AGENT_RUNTIME_MAX_CONCURRENCY"), 4),
            deepagent_concurrency=_int(values.get("AGENT_RUNTIME_DEEPAGENT_CONCURRENCY"), 1),
            max_pending=_int(values.get("AGENT_RUNTIME_MAX_PENDING"), 16),
            max_request_bytes=_int(values.get("AGENT_RUNTIME_MAX_REQUEST_BYTES"), 8 * MIB),
            max_event_bytes=_int(values.get("AGENT_RUNTIME_MAX_EVENT_BYTES"), 4 * MIB),
            max_artifact_bytes=_int(values.get("AGENT_RUNTIME_MAX_ARTIFACT_BYTES"), 2 * MIB),
            max_output_bytes=_int(values.get("AGENT_RUNTIME_MAX_OUTPUT_BYTES"), MIB),
            event_buffer_size=_int(values.get("AGENT_RUNTIME_EVENT_BUFFER_SIZE"), 16),
            shutdown_grace_seconds=_float(values.get("AGENT_RUNTIME_SHUTDOWN_GRACE_SECONDS"), 10.0),
            work_dir=Path(_string(values.get("AGENT_RUNTIME_WORK_DIR")) or "runs/runtime"),
            enable_fake_executor=_bool(values.get("AGENT_RUNTIME_ENABLE_FAKE_EXECUTOR"), False),
            collaboration_enabled=_bool(values.get("COLLABORATION_RUNTIME_ENABLED"), True),
            collaboration_engine_allowlist=_split_comma_list(
                values.get("COLLABORATION_ENGINE_ALLOWLIST"), ("native",)
            ),
            collaboration_autogen_enabled=_bool(
                values.get("COLLABORATION_AUTOGEN_ENABLED"), False
            ),
            collaboration_max_concurrency=_int(
                values.get("COLLABORATION_MAX_CONCURRENCY"), 4
            ),
            collaboration_max_pending=_int(
                values.get("COLLABORATION_MAX_PENDING"), 16
            ),
            collaboration_checkpoint_max_bytes=_int(
                values.get("COLLABORATION_CHECKPOINT_MAX_BYTES"), MIB
            ),
            collaboration_default_engine=_string(
                values.get("COLLABORATION_DEFAULT_ENGINE")
            )
            or "native",
            collaboration_default_trigger_mode=_string(
                values.get("COLLABORATION_DEFAULT_TRIGGER_MODE")
            )
            or "mention_only",
        )
        settings.validate()
        return settings


def _string(value: object) -> str:
    return str(value).strip() if value is not None else ""


def _int(value: object, default: int) -> int:
    text = _string(value)
    try:
        return int(text) if text else default
    except ValueError as exc:
        raise RuntimeConfigError(f"expected integer, got {text!r}") from exc


def _float(value: object, default: float) -> float:
    text = _string(value)
    try:
        return float(text) if text else default
    except ValueError as exc:
        raise RuntimeConfigError(f"expected number, got {text!r}") from exc


def _bool(value: object, default: bool) -> bool:
    text = _string(value).lower()
    if not text:
        return default
    if text in {"1", "true", "yes", "on"}:
        return True
    if text in {"0", "false", "no", "off"}:
        return False
    raise RuntimeConfigError(f"expected boolean, got {text!r}")


def _path(value: object) -> Path | None:
    text = _string(value)
    return Path(text) if text else None


def _split_comma_list(value: object, default: tuple[str, ...]) -> tuple[str, ...]:
    text = _string(value)
    if not text:
        return default
    items = tuple(item for item in (s.strip() for s in text.split(",")) if item)
    return items or default


def _is_loopback(host: str) -> bool:
    return host.strip().lower() in {"127.0.0.1", "::1", "localhost"}
