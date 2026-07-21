from __future__ import annotations

import logging
import os
import re
from collections.abc import Iterable, Mapping


SENSITIVE_NAME = re.compile(r"(api[_-]?key|authorization|password|passcode|secret|token|dsn)", re.I)
TOKEN_VALUE = re.compile(r"(?i)(bearer\s+|api[_-]?key[=:]\s*)([^\s,;]+)")


def environment_secrets(env: Mapping[str, str] | None = None) -> tuple[str, ...]:
    values = os.environ if env is None else env
    return tuple(
        value
        for name, value in values.items()
        if value and SENSITIVE_NAME.search(name) and len(value) >= 4
    )


def redact_text(value: object, secrets: Iterable[str] = ()) -> str:
    text = str(value)
    for secret in secrets:
        if secret:
            text = text.replace(secret, "[REDACTED]")
    return TOKEN_VALUE.sub(lambda match: match.group(1) + "[REDACTED]", text)


class SensitiveDataFilter(logging.Filter):
    """Redact known process credentials before a record reaches any handler."""

    def __init__(self, secrets: Iterable[str] = ()) -> None:
        super().__init__()
        self._secrets = tuple(secrets) + environment_secrets()

    def filter(self, record: logging.LogRecord) -> bool:
        record.msg = redact_text(record.getMessage(), self._secrets)
        record.args = ()
        if record.exc_info is not None:
            record.exc_info = None
            record.exc_text = None
        for name, value in list(vars(record).items()):
            if SENSITIVE_NAME.search(name):
                setattr(record, name, "[REDACTED]")
            elif isinstance(value, (str, Exception)):
                setattr(record, name, redact_text(value, self._secrets))
        return True


def install_sensitive_data_filter(secrets: Iterable[str] = ()) -> None:
    redactor = SensitiveDataFilter(secrets)
    root = logging.getLogger()
    for handler in root.handlers:
        handler.addFilter(redactor)
