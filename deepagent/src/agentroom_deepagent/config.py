from __future__ import annotations

import os
import tomllib
from dataclasses import dataclass
from pathlib import Path
from typing import Mapping

from dotenv import dotenv_values


DEFAULT_MODEL = "openai:gpt-5.5"
DEFAULT_SEARCH_MAX_RESULTS = 5
DEFAULT_SEARCH_TOPIC = "general"
PROJECT_DIR = Path(__file__).resolve().parents[2]
PROVIDER_API_KEYS = {
    "openai": "OPENAI_API_KEY",
    "anthropic": "ANTHROPIC_API_KEY",
    "google_genai": "GOOGLE_API_KEY",
    "google-genai": "GOOGLE_API_KEY",
    "google": "GOOGLE_API_KEY",
    "openrouter": "OPENROUTER_API_KEY",
    "fireworks": "FIREWORKS_API_KEY",
    "baseten": "BASETEN_API_KEY",
    "ollama": "OLLAMA_API_KEY",
    "azure_openai": "AZURE_OPENAI_API_KEY",
}


class MissingCredentials(RuntimeError):
    """Raised when required runtime credentials are absent."""


@dataclass(frozen=True)
class CustomEndpoint:
    enabled: bool
    protocol: str
    base_url: str
    api_key: str
    model_name: str


@dataclass(frozen=True)
class Settings:
    config_path: Path
    env_path: Path
    model_name: str
    search_max_results: int
    search_topic: str
    include_raw_content: bool
    output_dir: Path
    stream_updates: bool
    custom: CustomEndpoint
    env: Mapping[str, str]

    def validate_credentials(self) -> None:
        missing: list[str] = []
        if self.custom.enabled:
            if not self.custom.base_url:
                missing.append("MODEL_BASE_URL (or CUSTOM_BASE_URL)")
            if not self.custom.api_key:
                missing.append("MODEL_API_KEY (or CUSTOM_API_KEY)")
            effective_model = self.custom.model_name or self.model_name
            if not effective_model:
                missing.append("MODEL_NAME (or [model].name)")
        else:
            provider_key = provider_api_key_name(self.model_name)
            if provider_key and not self.env.get(provider_key):
                missing.append(provider_key)
        if not self.env.get("TAVILY_API_KEY"):
            missing.append("TAVILY_API_KEY")
        if missing:
            names = ", ".join(missing)
            raise MissingCredentials(f"missing required environment variable(s): {names}")


def load_settings(path: str | Path = "deepagent.toml", env: Mapping[str, str] | None = None) -> Settings:
    config_path = resolve_config_path(path)
    runtime_env = _build_runtime_env(config_path, env)
    data = _read_toml(config_path)
    base_dir = config_path.parent
    env_path = base_dir / ".env"

    model = data.get("model", {})
    search = data.get("search", {})
    runtime = data.get("runtime", {})
    custom_raw = model.get("custom", {})

    model_name = (
        _string(runtime_env.get("MODEL_NAME"))
        or _string(runtime_env.get("DEEPAGENT_MODEL"))
        or _string(model.get("name"))
        or DEFAULT_MODEL
    )
    runtime_env = _with_model_api_key_alias(runtime_env, model_name, sync_process_env=env is None)
    search_max_results = _int(
        runtime_env.get("RESEARCH_MAX_RESULTS"),
        _int(search.get("max_results"), DEFAULT_SEARCH_MAX_RESULTS),
    )
    search_topic = _string(runtime_env.get("RESEARCH_TOPIC")) or _string(search.get("topic")) or DEFAULT_SEARCH_TOPIC
    include_raw_content = _bool(
        runtime_env.get("RESEARCH_INCLUDE_RAW_CONTENT"),
        _bool(search.get("include_raw_content"), False),
    )
    output_dir = Path(_string(runtime_env.get("RESEARCH_OUTPUT_DIR")) or _string(runtime.get("output_dir")) or "runs")
    if not output_dir.is_absolute():
        output_dir = base_dir / output_dir

    stream_updates = _bool(
        runtime_env.get("RESEARCH_STREAM_UPDATES"),
        _bool(runtime.get("stream_updates"), True),
    )

    custom_base_url = (
        _string(runtime_env.get("MODEL_BASE_URL"))
        or _string(runtime_env.get("CUSTOM_BASE_URL"))
        or _string(custom_raw.get("base_url"))
    )
    custom_api_key = (
        _string(runtime_env.get("MODEL_API_KEY"))
        or _string(runtime_env.get("CUSTOM_API_KEY"))
        or _string(custom_raw.get("api_key"))
    )
    custom_model_name = (
        _string(runtime_env.get("MODEL_NAME"))
        or _string(runtime_env.get("CUSTOM_MODEL_NAME"))
        or _string(custom_raw.get("model_name"))
    )
    custom = CustomEndpoint(
        enabled=bool(custom_base_url)
        or _bool(runtime_env.get("CUSTOM_ENABLED"), _bool(custom_raw.get("enabled"), False)),
        protocol=_string(runtime_env.get("MODEL_PROTOCOL"))
        or _string(runtime_env.get("CUSTOM_PROTOCOL"))
        or _string(custom_raw.get("protocol"))
        or "openai",
        base_url=custom_base_url,
        api_key=custom_api_key,
        model_name=custom_model_name,
    )

    return Settings(
        config_path=config_path,
        env_path=env_path,
        model_name=model_name,
        search_max_results=search_max_results,
        search_topic=search_topic,
        include_raw_content=include_raw_content,
        output_dir=output_dir,
        stream_updates=stream_updates,
        custom=custom,
        env=runtime_env,
    )


def resolve_config_path(path: str | Path) -> Path:
    requested = Path(path)
    if requested.is_absolute():
        return requested

    cwd_path = requested.resolve()
    if cwd_path.exists():
        return cwd_path

    project_path = PROJECT_DIR / requested
    if project_path.exists():
        return project_path.resolve()

    return cwd_path


def _build_runtime_env(config_path: Path, env: Mapping[str, str] | None) -> Mapping[str, str]:
    if env is not None:
        return dict(env)

    env_path = config_path.parent / ".env"
    merged: dict[str, str] = {}
    if env_path.exists():
        merged.update(
            {
                key: value
                for key, value in dotenv_values(env_path).items()
                if value is not None
            }
        )

    for key, value in os.environ.items():
        if value:
            merged[key] = value
        elif key not in merged:
            merged[key] = value

    return merged


def _with_model_api_key_alias(
    env: Mapping[str, str],
    model_name: str,
    *,
    sync_process_env: bool,
) -> Mapping[str, str]:
    merged = dict(env)
    model_api_key = _string(merged.get("MODEL_API_KEY"))
    provider_key = provider_api_key_name(model_name)

    if model_api_key and provider_key and not merged.get(provider_key):
        merged[provider_key] = model_api_key

    if sync_process_env:
        for key in PROVIDER_API_KEYS.values():
            value = _string(merged.get(key))
            if value and not os.environ.get(key):
                os.environ[key] = value

    return merged


def provider_api_key_name(model_name: str) -> str:
    provider = model_name.split(":", 1)[0].strip().lower()
    return PROVIDER_API_KEYS.get(provider, "")


def _read_toml(path: Path) -> dict:
    if not path.exists():
        return {}
    with path.open("rb") as handle:
        return tomllib.load(handle)


def _string(value: object) -> str:
    if value is None:
        return ""
    return str(value).strip()


def _int(value: object, default: int) -> int:
    if value is None or value == "":
        return default
    return int(value)


def _bool(value: object, default: bool) -> bool:
    if value is None or value == "":
        return default
    if isinstance(value, bool):
        return value
    return str(value).strip().lower() in {"1", "true", "yes", "on"}
