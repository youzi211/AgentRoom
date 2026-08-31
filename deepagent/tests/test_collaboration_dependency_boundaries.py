import ast
from pathlib import Path


COLLABORATION_ROOT = Path(__file__).parents[1] / "src" / "collaboration_runtime"
BOUNDARY_FILES = (
    COLLABORATION_ROOT / "executor.py",
    COLLABORATION_ROOT / "model_client.py",
    COLLABORATION_ROOT / "model_gateway.py",
)
FORBIDDEN_IMPORT_ROOTS = {
    "aiohttp",
    "anthropic",
    "grpc",
    "httpx",
    "langchain_anthropic",
    "langchain_openai",
    "openai",
    "requests",
    "urllib",
}
FORBIDDEN_SOURCE_TERMS = {
    "api_key",
    "authorization",
    "localhost",
}


def production_boundary_files():
    engine_files = tuple((COLLABORATION_ROOT / "engines").rglob("*.py"))
    return BOUNDARY_FILES + engine_files


def test_collaboration_engines_do_not_bypass_the_model_gateway():
    violations = []

    for path in production_boundary_files():
        source = path.read_text(encoding="utf-8")
        tree = ast.parse(source, filename=str(path))
        for node in ast.walk(tree):
            imported = []
            if isinstance(node, ast.Import):
                imported = [alias.name for alias in node.names]
            elif isinstance(node, ast.ImportFrom) and node.module:
                imported = [node.module]
            for module in imported:
                root = module.split(".", 1)[0]
                if root in FORBIDDEN_IMPORT_ROOTS:
                    violations.append(f"{path.name}: forbidden import {module}")

        lowered = source.lower()
        for term in FORBIDDEN_SOURCE_TERMS:
            if term in lowered:
                violations.append(f"{path.name}: forbidden source term {term}")

    assert violations == []
