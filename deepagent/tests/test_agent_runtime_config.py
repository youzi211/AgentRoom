from pathlib import Path

import pytest

from agent_runtime.config import MIB, RuntimeConfigError, RuntimeSettings


def test_runtime_settings_require_explicit_insecure_or_tls():
    with pytest.raises(RuntimeConfigError, match="TLS certificate"):
        RuntimeSettings.from_env({})


def test_runtime_settings_load_safe_local_development_values(tmp_path):
    settings = RuntimeSettings.from_env(
        {
            "AGENT_RUNTIME_HOST": "127.0.0.1",
            "AGENT_RUNTIME_PORT": "0",
            "AGENT_RUNTIME_INSECURE": "true",
            "AGENT_RUNTIME_MAX_CONCURRENCY": "3",
            "AGENT_RUNTIME_DEEPAGENT_CONCURRENCY": "1",
            "AGENT_RUNTIME_MAX_PENDING": "2",
            "AGENT_RUNTIME_WORK_DIR": str(tmp_path),
        }
    )

    assert settings.bind_address == "127.0.0.1:0"
    assert settings.max_concurrency == 3
    assert settings.max_request_bytes == 8 * MIB
    assert settings.work_dir == Path(tmp_path)


def test_runtime_settings_reject_deepagent_capacity_above_total():
    with pytest.raises(RuntimeConfigError, match="cannot exceed"):
        RuntimeSettings.from_env(
            {
                "AGENT_RUNTIME_INSECURE": "true",
                "AGENT_RUNTIME_MAX_CONCURRENCY": "1",
                "AGENT_RUNTIME_DEEPAGENT_CONCURRENCY": "2",
            }
        )


def test_runtime_settings_reject_unreadable_tls_material(monkeypatch, tmp_path):
    cert = tmp_path / "server.crt"
    key = tmp_path / "server.key"
    cert.write_text("cert", encoding="utf-8")
    key.write_text("key", encoding="utf-8")
    original = Path.read_bytes

    def fail_for_key(path):
        if path == key:
            raise PermissionError("denied")
        return original(path)

    monkeypatch.setattr(Path, "read_bytes", fail_for_key)
    with pytest.raises(RuntimeConfigError, match="not readable"):
        RuntimeSettings(
            host="runtime.internal",
            tls_cert_file=cert,
            tls_key_file=key,
        ).validate()
