from __future__ import annotations

import logging

from agent_runtime.security import SensitiveDataFilter, redact_text


class Capture(logging.Handler):
    def __init__(self):
        super().__init__()
        self.messages = []

    def emit(self, record):
        self.messages.append(self.format(record))


def test_sensitive_filter_redacts_message_fields_and_exception_stack():
    secret = "request-scoped-model-secret"
    capture = Capture()
    capture.addFilter(SensitiveDataFilter([secret]))
    logger = logging.getLogger("agent-runtime-security-test")
    logger.handlers = [capture]
    logger.propagate = False
    logger.setLevel(logging.INFO)

    try:
        raise RuntimeError(f"provider echoed {secret}")
    except RuntimeError:
        logger.exception("failed with %s", secret, extra={"api_key": secret})

    rendered = "\n".join(capture.messages)
    assert secret not in rendered
    assert "[REDACTED]" in rendered


def test_redact_text_hides_authorization_tokens():
    assert redact_text("Authorization=Bearer abcdef") == "Authorization=Bearer [REDACTED]"
