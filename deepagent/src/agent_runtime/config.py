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


def _is_loopback(host: str) -> bool:
    return host.strip().lower() in {"127.0.0.1", "::1", "localhost"}
