"""Chat model construction.

Per the DeepAgents docs, `create_deep_agent(model=...)` accepts either a
provider-prefixed model name string ("openai:gpt-5.5") or a LangChain
`BaseChatModel` object. We return a string for provider-native models and a
configured `ChatOpenAI`/`ChatAnthropic` object for OpenAI-compatible custom
endpoints.
"""

from __future__ import annotations

from typing import Union

from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from agentroom_deepagent.config import Settings


ChatModel = Union[str, "ChatOpenAI", "ChatAnthropic"]


def build_model(settings: Settings) -> ChatModel:
    """Build the chat model used by the research agent.

    Returns the provider-native model name as a string when no custom endpoint
    is configured (letting `create_deep_agent` resolve it), or a configured
    chat model object pointing at the custom endpoint otherwise.
    """
    if not settings.custom.enabled:
        return settings.model_name

    effective_model = settings.custom.model_name or settings.model_name
    if settings.custom.protocol.lower() == "anthropic":
        return ChatAnthropic(
            model=effective_model,
            base_url=settings.custom.base_url,
            api_key=settings.custom.api_key,
        )
    return ChatOpenAI(
        model=effective_model,
        base_url=settings.custom.base_url,
        api_key=settings.custom.api_key,
    )
