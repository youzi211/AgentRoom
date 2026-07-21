"""Stable protocol helpers shared by the Python runtime and contract tests."""

PROTOCOL_VERSION = "v1"


class ProtocolVersionError(ValueError):
    """Raised before execution when a client requests an unsupported contract."""


def validate_protocol_version(version: str) -> None:
    if version != PROTOCOL_VERSION:
        raise ProtocolVersionError(f"unsupported Agent Runtime protocol version {version!r}")
