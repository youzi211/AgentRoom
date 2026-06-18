"""Tool construction for the research DeepAgent.

DeepAgents infers a tool's schema from the callable's signature and docstring,
so `internet_search` is a plain function — it just happens to be produced by a
factory so its default arguments can be wired from `Settings`.
"""

from __future__ import annotations

from typing import Callable, Literal

from tavily import TavilyClient

from agentroom_deepagent.config import Settings


def build_search_tool(settings: Settings) -> Callable:
    """Return an `internet_search` callable backed by Tavily."""
    tavily_client = TavilyClient(api_key=settings.env["TAVILY_API_KEY"])

    def internet_search(
        query: str,
        max_results: int = settings.search_max_results,
        topic: Literal["general", "news", "finance"] = settings.search_topic,
        include_raw_content: bool = settings.include_raw_content,
    ):
        """Run a public web search for current research sources."""
        return tavily_client.search(
            query,
            max_results=max_results,
            topic=topic or settings.search_topic,
            include_raw_content=include_raw_content,
        )

    return internet_search
