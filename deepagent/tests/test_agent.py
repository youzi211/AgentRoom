from agentroom_deepagent import agent as agent_module


def test_create_research_agent_configures_research_subagent(monkeypatch):
    captured = {}

    def fake_create_deep_agent(**kwargs):
        captured.update(kwargs)
        return object()

    monkeypatch.setattr(agent_module, "create_deep_agent", fake_create_deep_agent)

    backend = object()
    search_tool = object()
    created = agent_module.create_research_agent("test-model", search_tool, backend=backend)

    assert created is not None
    assert captured["model"] == "test-model"
    assert captured["tools"] == [search_tool]
    assert captured["backend"] is backend
    assert len(captured["subagents"]) == 1
    subagent = captured["subagents"][0]
    assert subagent["name"] == "research-agent"
    assert "research" in subagent["description"].lower()
    assert subagent["tools"] == [search_tool]
