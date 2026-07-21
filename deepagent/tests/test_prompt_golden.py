import json
from pathlib import Path

from google.protobuf.json_format import ParseDict

from agent_runtime.prompt import compose_prompt
from agent_runtime.v1 import agent_runtime_pb2


FIXTURE = (
    Path(__file__).parents[2]
    / "proto"
    / "agent_runtime"
    / "v1"
    / "testdata"
    / "prompt_golden.json"
)


def test_python_prompt_composer_matches_cross_language_golden_samples():
    cases = json.loads(FIXTURE.read_text(encoding="utf-8"))

    for case in cases:
        request = ParseDict(case["request"], agent_runtime_pb2.ExecuteAgentRequest())
        actual = [message.__dict__ for message in compose_prompt(request)]
        assert actual == case["expected"], case["name"]
