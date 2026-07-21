from agent_runtime import prompt
from agent_runtime.v1 import agent_runtime_pb2


def _message(sender_name: str, content: str, sender_type=agent_runtime_pb2.SENDER_TYPE_HUMAN):
    return agent_runtime_pb2.MessageSnapshot(
        sender_name=sender_name,
        sender_type=sender_type,
        content=content,
    )


def test_transcript_short_message_renders_in_full():
    messages = [_message("Alice", "A short message.")]
    rendered = prompt.format_transcript(messages)
    assert "Alice (human): A short message." in rendered


def test_transcript_overlong_recent_message_is_truncated_with_marker():
    long_content = "x" * 801
    messages = [_message("Alice", long_content)]
    rendered = prompt.format_transcript(messages)
    assert long_content not in rendered
    assert "…[消息过长，已截断]" in rendered
    assert "x" * 800 in rendered


def test_transcript_message_at_exact_char_limit_is_not_truncated():
    exact_content = "y" * 800
    messages = [_message("Alice", exact_content)]
    rendered = prompt.format_transcript(messages)
    assert "已截断" not in rendered
    assert exact_content in rendered


def test_transcript_older_messages_downgrade_to_summary_line():
    messages = [
        _message(
            "Alice",
            "Full detail content for message number that is reasonably long to test summarization behavior "
            + chr(ord("A") + i),
        )
        for i in range(12)
    ]
    rendered = prompt.format_transcript(messages)

    assert "Full detail content for message number that is reasonably long to test summariza…" in rendered
    assert "Full detail content for message number that is reasonably long to test summarization behavior L" in rendered


def test_transcript_empty_produces_none_marker():
    rendered = prompt.format_transcript([])
    assert rendered == "Visible room transcript:\n- none"


def test_transcript_budget_drops_oldest_messages_when_over_budget():
    messages = [_message("Alice", "z" * 800) for _ in range(30)]
    rendered = prompt.format_transcript(messages)

    line_count = rendered.count("Alice (human):")
    assert line_count < 30
    assert line_count >= 5


def test_transcript_budget_stops_at_minimum_retained_messages():
    messages = [_message("Alice", "w" * 800) for _ in range(5)]
    rendered = prompt.format_transcript(messages)

    line_count = rendered.count("Alice (human):")
    assert line_count == 5
