"""DeepAgent factory.

This module does one thing: build the research DeepAgent from an already
constructed chat model and search tool. It knows nothing about Settings,
Tavily, or how the model is wired - those are the responsibilities of
models.py, tools.py, and research.py respectively.
"""

from __future__ import annotations

from typing import Callable

from deepagents import create_deep_agent

from agentroom_deepagent.prompts import RESEARCH_INSTRUCTIONS, RESEARCH_SUBAGENT_PROMPT


def create_research_agent(model, search_tool: Callable, *, backend=None):
    """Create the research DeepAgent.

    Args:
        model: A provider-prefixed model name string or a LangChain
            BaseChatModel object (see models.build_model).
        search_tool: The internet_search callable (see tools.build_search_tool).
    """
    research_subagent = {
        "name": "research-agent",
        "description": "Researches public web sources and writes the final Markdown report.",
        "system_prompt": RESEARCH_SUBAGENT_PROMPT,
        "tools": [search_tool],
    }

    return create_deep_agent(
        model=model,
        tools=[search_tool],
        system_prompt=RESEARCH_INSTRUCTIONS,
        subagents=[research_subagent],
        backend=backend,
    )
