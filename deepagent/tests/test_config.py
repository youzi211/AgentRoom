import textwrap

import pytest

from agentroom_deepagent.config import MissingCredentials, load_settings, resolve_config_path


def test_load_settings_reads_config_file_and_environment_overrides(tmp_path):
    config_path = tmp_path / "deepagent.toml"
    config_path.write_text(
        textwrap.dedent(
            """
            [model]
            name = "openai:gpt-5.5"

            [search]
            max_results = 7
            topic = "news"
            include_raw_content = true

            [runtime]
            output_dir = "custom-runs"
            stream_updates = false
            """
        ),
        encoding="utf-8",
    )

    settings = load_settings(
        config_path,
        env={
            "DEEPAGENT_MODEL": "anthropic:claude-sonnet-4-6",
            "RESEARCH_MAX_RESULTS": "3",
        },
    )

    assert settings.model_name == "anthropic:claude-sonnet-4-6"
    assert settings.search_max_results == 3
    assert settings.search_topic == "news"
    assert settings.include_raw_content is True
    assert settings.output_dir == tmp_path / "custom-runs"
    assert settings.stream_updates is False


def test_load_settings_uses_safe_defaults_when_config_is_missing(tmp_path):
    settings = load_settings(tmp_path / "missing.toml", env={})

    assert settings.model_name == "openai:gpt-5.5"
    assert settings.search_max_results == 5
    assert settings.search_topic == "general"
    assert settings.include_raw_content is False
    assert settings.output_dir == tmp_path / "runs"
    assert settings.stream_updates is True


def test_load_settings_reads_dotenv_next_to_config_when_env_is_not_injected(tmp_path, monkeypatch):
    for name in ("OPENAI_API_KEY", "TAVILY_API_KEY"):
        monkeypatch.delenv(name, raising=False)

    config_path = tmp_path / "deepagent.toml"
    config_path.write_text('[model]\nname = "openai:gpt-5.5"\n', encoding="utf-8")
    (tmp_path / ".env").write_text(
        "OPENAI_API_KEY=from-dotenv\nTAVILY_API_KEY=from-dotenv\n",
        encoding="utf-8",
    )

    settings = load_settings(config_path)

    assert settings.env["OPENAI_API_KEY"] == "from-dotenv"
    assert settings.env["TAVILY_API_KEY"] == "from-dotenv"
    settings.validate_credentials()


def test_load_settings_uses_dotenv_when_process_env_has_empty_values(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "")
    monkeypatch.setenv("TAVILY_API_KEY", "")

    config_path = tmp_path / "deepagent.toml"
    config_path.write_text('[model]\nname = "openai:gpt-5.5"\n', encoding="utf-8")
    (tmp_path / ".env").write_text(
        "OPENAI_API_KEY=from-dotenv\nTAVILY_API_KEY=from-dotenv\n",
        encoding="utf-8",
    )

    settings = load_settings(config_path)

    assert settings.env["OPENAI_API_KEY"] == "from-dotenv"
    assert settings.env["TAVILY_API_KEY"] == "from-dotenv"
    settings.validate_credentials()


def test_non_empty_process_model_environment_overrides_dotenv_and_toml(tmp_path, monkeypatch):
    config_path = tmp_path / "deepagent.toml"
    config_path.write_text(
        '[model]\nname = "toml-model"\n[model.custom]\nenabled = true\nbase_url = "https://toml.example/v1"\napi_key = "toml-secret"\n',
        encoding="utf-8",
    )
    (tmp_path / ".env").write_text(
        "MODEL_PROTOCOL=openai\nMODEL_BASE_URL=https://dotenv.example/v1\nMODEL_NAME=dotenv-model\nMODEL_API_KEY=dotenv-secret\n",
        encoding="utf-8",
    )
    monkeypatch.setenv("MODEL_PROTOCOL", "openai")
    monkeypatch.setenv("MODEL_BASE_URL", "https://database-injected.example/v1")
    monkeypatch.setenv("MODEL_NAME", "database-injected-model")
    monkeypatch.setenv("MODEL_API_KEY", "database-injected-secret")

    settings = load_settings(config_path)

    assert settings.custom.base_url == "https://database-injected.example/v1"
    assert settings.custom.model_name == "database-injected-model"
    assert settings.custom.api_key == "database-injected-secret"
    assert "database-injected-secret" not in config_path.read_text(encoding="utf-8")
    assert "database-injected-secret" not in (tmp_path / ".env").read_text(encoding="utf-8")


def test_model_endpoint_can_be_configured_entirely_from_dotenv(tmp_path, monkeypatch):
    for name in ("MODEL_PROTOCOL", "MODEL_BASE_URL", "MODEL_NAME", "MODEL_API_KEY", "TAVILY_API_KEY"):
        monkeypatch.delenv(name, raising=False)

    config_path = tmp_path / "deepagent.toml"
    config_path.write_text("[runtime]\noutput_dir = \"runs\"\n", encoding="utf-8")
    (tmp_path / ".env").write_text(
        "\n".join(
            [
                "MODEL_PROTOCOL=openai",
                "MODEL_BASE_URL=https://model-gateway.example.com/v1",
                "MODEL_NAME=research-model",
                "MODEL_API_KEY=model-key",
                "TAVILY_API_KEY=tavily-key",
            ]
        )
        + "\n",
        encoding="utf-8",
    )

    settings = load_settings(config_path)

    assert settings.model_name == "research-model"
    assert settings.custom.enabled is True
    assert settings.custom.protocol == "openai"
    assert settings.custom.base_url == "https://model-gateway.example.com/v1"
    assert settings.custom.api_key == "model-key"
    assert settings.custom.model_name == "research-model"
    settings.validate_credentials()


def test_model_api_key_alias_satisfies_provider_key_when_base_url_is_absent(tmp_path, monkeypatch):
    for name in ("MODEL_NAME", "MODEL_API_KEY", "OPENAI_API_KEY", "TAVILY_API_KEY"):
        monkeypatch.delenv(name, raising=False)

    config_path = tmp_path / "deepagent.toml"
    config_path.write_text("[model]\nname = \"openai:gpt-5.5\"\n", encoding="utf-8")
    (tmp_path / ".env").write_text(
        "MODEL_API_KEY=model-key\nTAVILY_API_KEY=tavily-key\n",
        encoding="utf-8",
    )

    settings = load_settings(config_path)

    assert settings.env["OPENAI_API_KEY"] == "model-key"
    settings.validate_credentials()


def test_resolve_config_path_finds_project_default_from_other_working_directory(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)

    resolved = resolve_config_path("deepagent.toml")

    assert resolved.name == "deepagent.toml"
    assert resolved.parent.name == "deepagent"
    assert resolved.exists()


def test_validate_credentials_reports_all_missing_keys(tmp_path):
    settings = load_settings(tmp_path / "missing.toml", env={})

    with pytest.raises(MissingCredentials) as exc_info:
        settings.validate_credentials()

    message = str(exc_info.value)
    assert "OPENAI_API_KEY" in message
    assert "TAVILY_API_KEY" in message


def test_validate_credentials_accepts_provider_key_and_tavily_key(tmp_path):
    settings = load_settings(
        tmp_path / "missing.toml",
        env={
            "DEEPAGENT_MODEL": "anthropic:claude-sonnet-4-6",
            "ANTHROPIC_API_KEY": "test-anthropic",
            "TAVILY_API_KEY": "test-tavily",
        },
    )

    settings.validate_credentials()


def test_custom_endpoint_defaults_to_disabled(tmp_path):
    settings = load_settings(tmp_path / "missing.toml", env={"TAVILY_API_KEY": "t", "OPENAI_API_KEY": "o"})

    assert settings.custom.enabled is False
    assert settings.custom.protocol == "openai"
    assert settings.custom.base_url == ""
    assert settings.custom.api_key == ""
    assert settings.custom.model_name == ""


def test_custom_endpoint_from_env(tmp_path):
    settings = load_settings(
        tmp_path / "missing.toml",
        env={
            "CUSTOM_ENABLED": "true",
            "CUSTOM_PROTOCOL": "anthropic",
            "CUSTOM_BASE_URL": "https://my-proxy.example.com/v1",
            "CUSTOM_API_KEY": "sk-custom",
            "CUSTOM_MODEL_NAME": "my-model",
            "TAVILY_API_KEY": "t",
        },
    )

    assert settings.custom.enabled is True
    assert settings.custom.protocol == "anthropic"
    assert settings.custom.base_url == "https://my-proxy.example.com/v1"
    assert settings.custom.api_key == "sk-custom"
    assert settings.custom.model_name == "my-model"


def test_custom_endpoint_from_toml(tmp_path):
    config_path = tmp_path / "deepagent.toml"
    config_path.write_text(
        textwrap.dedent(
            """
            [model]
            name = "openai:gpt-5.5"

            [model.custom]
            enabled = true
            protocol = "openai"
            base_url = "https://local-llm:8080/v1"
            api_key = "local-key"
            model_name = "qwen3"
            """
        ),
        encoding="utf-8",
    )

    settings = load_settings(config_path, env={"TAVILY_API_KEY": "t"})

    assert settings.custom.enabled is True
    assert settings.custom.protocol == "openai"
    assert settings.custom.base_url == "https://local-llm:8080/v1"
    assert settings.custom.api_key == "local-key"
    assert settings.custom.model_name == "qwen3"


def test_validate_credentials_requires_custom_fields_when_enabled(tmp_path):
    settings = load_settings(
        tmp_path / "missing.toml",
        env={
            "CUSTOM_ENABLED": "true",
            "TAVILY_API_KEY": "t",
        },
    )

    with pytest.raises(MissingCredentials) as exc_info:
        settings.validate_credentials()

    message = str(exc_info.value)
    assert "MODEL_BASE_URL" in message
    assert "MODEL_API_KEY" in message
