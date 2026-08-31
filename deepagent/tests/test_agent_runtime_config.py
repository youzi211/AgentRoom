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


def test_runtime_settings_load_collaboration_controls_from_env():
    settings = RuntimeSettings.from_env(
        {
            "AGENT_RUNTIME_INSECURE": "true",
            "COLLABORATION_RUNTIME_ENABLED": "false",
            "COLLABORATION_ENGINE_ALLOWLIST": "native, autogen",
            "COLLABORATION_AUTOGEN_ENABLED": "true",
            "COLLABORATION_MAX_CONCURRENCY": "2",
            "COLLABORATION_MAX_PENDING": "3",
            "COLLABORATION_CHECKPOINT_MAX_BYTES": "4096",
            "COLLABORATION_DEFAULT_ENGINE": "autogen",
            "COLLABORATION_DEFAULT_TRIGGER_MODE": "automatic",
        }
    )

    assert settings.collaboration_enabled is False
    assert settings.collaboration_engine_allowlist == ("native", "autogen")
    assert settings.collaboration_autogen_enabled is True
    assert settings.collaboration_max_concurrency == 2
    assert settings.collaboration_max_pending == 3
    assert settings.collaboration_checkpoint_max_bytes == 4096
    assert settings.collaboration_default_engine == "autogen"
    assert settings.collaboration_default_trigger_mode == "automatic"


def test_runtime_settings_reject_collaboration_default_outside_allowlist():
    with pytest.raises(RuntimeConfigError, match="DEFAULT_ENGINE"):
        RuntimeSettings.from_env(
            {
                "AGENT_RUNTIME_INSECURE": "true",
                "COLLABORATION_ENGINE_ALLOWLIST": "native",
                "COLLABORATION_DEFAULT_ENGINE": "autogen",
            }
        )
