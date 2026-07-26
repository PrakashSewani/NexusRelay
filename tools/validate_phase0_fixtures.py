#!/usr/bin/env python3
"""Validate repository-pinned Phase 0 OpenCode and OpenAI wire fixtures."""

import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any, Dict, Iterable, List, Sequence, Tuple


ROOT = Path(__file__).resolve().parents[1]
OPENCODE = ROOT / "docs/agents/fixtures/opencode"
OPENAI = ROOT / "docs/testing/fixtures/openai-sdk"


class ValidationError(Exception):
    pass


def fail(message: str) -> None:
    raise ValidationError(message)


def rel(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def read_text(path: Path) -> str:
    data = path.read_bytes()
    if data.startswith(b"\xef\xbb\xbf"):
        fail(f"{rel(path)}: UTF-8 BOM is not allowed")
    if b"\r" in data:
        fail(f"{rel(path)}: CR/CRLF line endings are not allowed")
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as error:
        fail(f"{rel(path)}: invalid UTF-8: {error}")
    if not text.endswith("\n"):
        fail(f"{rel(path)}: file must end with LF")
    for number, line in enumerate(text.splitlines(), 1):
        if line.endswith((" ", "\t")):
            fail(f"{rel(path)}:{number}: trailing whitespace")
    return text


def load_json(path: Path) -> Any:
    text = read_text(path)
    try:
        value = json.loads(text)
    except json.JSONDecodeError as error:
        fail(f"{rel(path)}: invalid JSON: {error}")
    expected = json.dumps(value, ensure_ascii=False, indent=2) + "\n"
    if text != expected:
        fail(f"{rel(path)}: JSON is not canonical two-space-indented serialization")
    return value


def load_jsonl(path: Path) -> List[Any]:
    text = read_text(path)
    records = []
    for number, line in enumerate(text.splitlines(), 1):
        if not line:
            fail(f"{rel(path)}:{number}: empty JSONL record")
        try:
            value = json.loads(line)
        except json.JSONDecodeError as error:
            fail(f"{rel(path)}:{number}: invalid JSON: {error}")
        expected = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
        if line != expected:
            fail(f"{rel(path)}:{number}: JSONL record is not compact canonical JSON")
        records.append(value)
    return records


def parse_sse(path: Path) -> List[Tuple[str, Any, str]]:
    text = read_text(path)
    if not text.endswith("\n\n") or text.endswith("\n\n\n"):
        fail(f"{rel(path)}: SSE must end after exactly one empty LF line")
    records = []
    for index, block in enumerate(text[:-2].split("\n\n"), 1):
        lines = block.split("\n")
        event = ""
        data_line = ""
        if lines[0].startswith("event: "):
            if len(lines) != 2:
                fail(f"{rel(path)}: SSE record {index} must have one event and one data line")
            event = lines[0][7:]
            data_line = lines[1]
        elif len(lines) == 1:
            data_line = lines[0]
        else:
            fail(f"{rel(path)}: SSE record {index} has invalid line structure")
        if not data_line.startswith("data: "):
            fail(f"{rel(path)}: SSE record {index} is missing data line")
        raw = data_line[6:]
        if raw == "[DONE]":
            value = raw
        else:
            try:
                value = json.loads(raw)
            except json.JSONDecodeError as error:
                fail(f"{rel(path)}: SSE record {index} has invalid JSON: {error}")
            if raw != json.dumps(value, ensure_ascii=False, separators=(",", ":")):
                fail(f"{rel(path)}: SSE record {index} data is not compact canonical JSON")
            if event and value.get("type") != event:
                fail(f"{rel(path)}: SSE event name does not match JSON type at record {index}")
        records.append((event, value, raw))
    return records


def verify_checksums(directory: Path, expected_files: Sequence[str]) -> None:
    checksum_path = directory / "SHA256SUMS"
    lines = read_text(checksum_path).splitlines()
    actual_names = []
    for number, line in enumerate(lines, 1):
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_./-]+)", line)
        if not match:
            fail(f"{rel(checksum_path)}:{number}: invalid checksum record")
        digest, name = match.groups()
        target = directory / name
        if not target.is_file() or directory.resolve() not in target.resolve().parents:
            fail(f"{rel(checksum_path)}:{number}: checksum target escapes or is missing from fixture tree")
        actual = hashlib.sha256(target.read_bytes()).hexdigest()
        if actual != digest:
            fail(f"{rel(target)}: SHA-256 mismatch, expected {digest}, got {actual}")
        actual_names.append(name)
    if list(expected_files) != actual_names:
        fail(f"{rel(checksum_path)}: checksum inventory/order differs from expected fixture inventory")


def walk_strings(value: Any) -> Iterable[str]:
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for key, child in value.items():
            yield str(key)
            yield from walk_strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_strings(child)


def walk_values(value: Any) -> Iterable[str]:
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for child in value.values():
            yield from walk_values(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_values(child)


def reject_secret_patterns(path: Path, values: Iterable[str]) -> None:
    patterns = (
        re.compile(r"\bsk-[A-Za-z0-9_-]{12,}"),
        re.compile(r"\bBearer\s+\S+", re.IGNORECASE),
        re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
        re.compile(r"(?i)(?:api[_-]?key|password|secret)\s*[=:]\s*[^{}\s\"']{8,}"),
    )
    for value in values:
        for pattern in patterns:
            if pattern.search(value):
                fail(f"{rel(path)}: possible plaintext secret found")


def validate_opencode() -> int:
    expected = ["minimal.json", "capabilities.json", "omissions.json", "project-override-threat.json"]
    actual_json = sorted(path.name for path in OPENCODE.glob("*.json"))
    if sorted(expected) != actual_json:
        fail("OpenCode fixture inventory differs from the four documented JSON fixtures")
    verify_checksums(OPENCODE, expected)

    for name in expected:
        path = OPENCODE / name
        value = load_json(path)
        reject_secret_patterns(path, walk_strings(value))
        if set(value) != {"$schema", "provider"}:
            fail(f"{rel(path)}: only $schema and provider are allowed at top level")
        if value["$schema"] != "https://opencode.ai/config.json":
            fail(f"{rel(path)}: unexpected schema reference")
        if list(value["provider"]) != ["nexusrelay"]:
            fail(f"{rel(path)}: expected exactly the nexusrelay provider")
        provider = value["provider"]["nexusrelay"]
        allowed_provider = {"name", "npm", "options", "models"}
        if not set(provider).issubset(allowed_provider) or not {"npm", "options", "models"}.issubset(provider):
            fail(f"{rel(path)}: invalid provider shape")
        if provider["npm"] != "@ai-sdk/openai-compatible":
            fail(f"{rel(path)}: provider npm package is not pinned")
        if set(provider["options"]) != {"baseURL", "apiKey"}:
            fail(f"{rel(path)}: options must contain only baseURL and apiKey")
        if provider["options"]["apiKey"] != "{env:NEXUSRELAY_API_KEY}":
            fail(f"{rel(path)}: apiKey must be the exact pinned environment reference")
        expected_url = "https://attacker.invalid/v1" if name == "project-override-threat.json" else "https://gateway.example.com/v1"
        if provider["options"]["baseURL"] != expected_url:
            fail(f"{rel(path)}: baseURL differs from the fixture contract")
        if not provider["models"] or not all(isinstance(key, str) and isinstance(model, dict) for key, model in provider["models"].items()):
            fail(f"{rel(path)}: models must be a non-empty object of model objects")
        if "enabled_providers" in value:
            fail(f"{rel(path)}: enabled_providers must not be emitted")

    minimal = load_json(OPENCODE / "minimal.json")
    if minimal["provider"]["nexusrelay"]["models"] != {"gateway-model": {}}:
        fail("OpenCode minimal fixture model shape changed")
    capabilities = load_json(OPENCODE / "capabilities.json")["provider"]["nexusrelay"]["models"]["vision-tools"]
    if capabilities.get("modalities") != {"input": ["text", "image"], "output": ["text"]}:
        fail("OpenCode capabilities fixture modalities changed")
    omissions = load_json(OPENCODE / "omissions.json")["provider"]["nexusrelay"]["models"]["metadata-unknown"]
    if omissions != {"name": "Metadata Unknown"}:
        fail("OpenCode omissions fixture must not guess capabilities, limits, or cost")
    return len(expected)


def require_keys(value: Dict[str, Any], keys: Sequence[str], label: str) -> None:
    missing = [key for key in keys if key not in value]
    if missing:
        fail(f"{label}: missing keys {', '.join(missing)}")


def validate_responses_usage(usage: Dict[str, Any], expected: Tuple[int, int, int], label: str) -> None:
    if set(usage) != {"input_tokens", "input_tokens_details", "output_tokens", "output_tokens_details", "total_tokens"}:
        fail(f"{label}: unexpected usage fields")
    if usage["input_tokens_details"] != {"cached_tokens": 0} or usage["output_tokens_details"] != {"reasoning_tokens": 0}:
        fail(f"{label}: unexpected usage detail structure")
    actual = (usage["input_tokens"], usage["output_tokens"], usage["total_tokens"])
    if actual != expected or not all(isinstance(value, int) and not isinstance(value, bool) for value in actual):
        fail(f"{label}: unexpected token counts")


def validate_chat_stream(records: List[Tuple[str, Any, str]], path: Path, success: bool) -> None:
    if any(event for event, _, _ in records):
        fail(f"{rel(path)}: Chat SSE must use data lines only")
    values = [value for _, value, _ in records]
    if success:
        if values[-1] != "[DONE]":
            fail(f"{rel(path)}: successful Chat stream must end with [DONE]")
        chunks = values[:-1]
        if len(chunks) < 3 or chunks[-2]["choices"][0]["finish_reason"] != "tool_calls":
            fail(f"{rel(path)}: Chat finish chunk is missing or out of order")
        if chunks[-1].get("choices") != [] or not isinstance(chunks[-1].get("usage"), dict):
            fail(f"{rel(path)}: Chat usage chunk must immediately precede [DONE]")
        if chunks[0]["choices"][0]["delta"] != {"role": "assistant"}:
            fail(f"{rel(path)}: Chat stream must begin with the assistant role delta")
    elif any(value == "[DONE]" or (isinstance(value, dict) and value.get("usage")) for value in values):
        fail(f"{rel(path)}: failed Chat stream must have neither [DONE] nor success usage")


def validate_responses_stream(records: List[Tuple[str, Any, str]], path: Path, failed: bool) -> None:
    values = [value for _, value, _ in records]
    types = [value["type"] for value in values]
    terminal = "response.failed" if failed else "response.completed"
    required_prefix = ["response.created", "response.in_progress", "response.output_item.added"]
    if types[:3] != required_prefix or types[-1] != terminal:
        fail(f"{rel(path)}: invalid Responses lifecycle ordering")
    if ("response.completed" in types) == failed or ("response.failed" in types) != failed:
        fail(f"{rel(path)}: invalid Responses terminal event coverage")
    sequences = [value.get("sequence_number") for value in values]
    if sequences != list(range(len(values))):
        fail(f"{rel(path)}: sequence_number must be contiguous from zero")
    response_ids = [value["response"]["id"] for value in values if "response" in value]
    if len(set(response_ids)) != 1:
        fail(f"{rel(path)}: response IDs are not stable")


def validate_openai() -> int:
    manifest = load_json(OPENAI / "manifest.json")
    observations = load_json(OPENAI / manifest["request_observations"])
    expected_pins = {
        "python": "openai==2.48.0",
        "javascript": "openai@6.49.0",
        "go": "github.com/openai/openai-go/v3@v3.46.0",
    }
    for language, dependency in expected_pins.items():
        if manifest["sdk_pins"][language]["dependency"] != dependency:
            fail(f"OpenAI manifest {language} SDK pin changed")

    fixture_names = [case["fixture"] for case in manifest["success_cases"]]
    fixture_names += [manifest["precommit_error_fixture"]]
    fixture_names += [case["fixture"] for case in manifest["postcommit_cases"]]
    fixture_names += [manifest["cancellation_fixture"], manifest["request_observations"]]
    disk_names = sorted(
        path.relative_to(OPENAI).as_posix()
        for path in OPENAI.rglob("*")
        if path.is_file() and path.name not in {"manifest.json", "SHA256SUMS"}
    )
    if sorted(fixture_names) != disk_names or len(fixture_names) != len(set(fixture_names)):
        fail("OpenAI manifest inventory does not exactly cover fixture files")
    verify_checksums(OPENAI, ["manifest.json"] + sorted(fixture_names))

    observation_ids = [item["id"] for item in observations["observations"]]
    success_ids = [item["id"] for item in manifest["success_cases"]]
    if observation_ids != success_ids:
        fail("SDK request observations do not match success-case inventory/order")
    if observations["applies_to"] != list(expected_pins.values()):
        fail("SDK request observations do not apply to the exact manifest pins")

    parsed: Dict[str, Any] = {}
    for case in manifest["success_cases"]:
        path = OPENAI / case["fixture"]
        parsed[case["id"]] = parse_sse(path) if path.suffix == ".sse" else load_json(path)
    errors = load_jsonl(OPENAI / manifest["precommit_error_fixture"])
    for case in manifest["postcommit_cases"]:
        parsed[case["id"]] = parse_sse(OPENAI / case["fixture"])
    cancellation = load_json(OPENAI / manifest["cancellation_fixture"])

    models = parsed["models_list"]
    if models.get("object") != "list" or [model["id"] for model in models.get("data", [])] != [
        "nr-chat-sentinel", "nr-responses-sentinel", "nr-embedding-sentinel"
    ]:
        fail("Models fixture must expose exactly the three gateway aliases in order")

    chat = parsed["chat_nonstream_tools"]
    choice = chat["choices"][0]
    call = choice["message"]["tool_calls"][0]
    if choice["finish_reason"] != "tool_calls" or call["function"]["arguments"] != '{"city":"NR_SENTINEL_CITY"}':
        fail("Chat non-stream tool-call structure changed")
    require_keys(chat["usage"], ["prompt_tokens", "completion_tokens", "total_tokens"], "Chat usage")
    validate_chat_stream(parsed["chat_stream_tools_usage"], OPENAI / "chat/stream-tools.response.sse", True)

    response = parsed["responses_nonstream_text"]
    if response.get("status") != "completed" or response.get("error") is not None or response.get("incomplete_details") is not None:
        fail("Responses non-stream completion state changed")
    if response["output"][0]["content"][0]["text"] != "NR_SENTINEL_OUTPUT_RESPONSE_TEXT":
        fail("Responses non-stream synthetic output changed")
    validate_responses_usage(response.get("usage", {}), (9, 5, 14), "Responses non-stream usage")
    validate_responses_stream(parsed["responses_stream_tools_usage"], OPENAI / "responses/stream-tools.response.sse", False)
    terminal_response = parsed["responses_stream_tools_usage"][-1][1]["response"]
    validate_responses_usage(terminal_response.get("usage", {}), (13, 6, 19), "Responses stream terminal usage")

    embeddings = parsed["embeddings_float"]
    if [item["index"] for item in embeddings["data"]] != [0, 1] or not all(isinstance(n, (int, float)) for item in embeddings["data"] for n in item["embedding"]):
        fail("Embeddings fixture order/vector structure changed")
    embedding_usage = embeddings.get("usage", {})
    if set(embedding_usage) != {"prompt_tokens", "total_tokens"} or embedding_usage != {"prompt_tokens": 6, "total_tokens": 6}:
        fail("Embeddings usage structure or token counts changed")

    expected_errors = ["authentication_error", "unknown_field", "unsupported_model_capability"]
    if len(errors) != len(manifest["precommit_error_records"]):
        fail("Pre-commit error record count differs from manifest")
    for index, (metadata, record) in enumerate(zip(manifest["precommit_error_records"], errors), 1):
        error = record.get("error", {})
        if metadata["line"] != index or metadata["id"] != expected_errors[index - 1]:
            fail("Pre-commit error manifest coverage/order changed")
        if error.get("code") != metadata["id"] or error.get("request_id") != metadata["x_request_id"]:
            fail(f"Pre-commit error line {index} does not match manifest metadata")
        if error.get("param", "absent") is None:
            fail(f"Pre-commit error line {index} emits param as null")

    validate_chat_stream(parsed["chat_postcommit_close"], OPENAI / "failures/chat-postcommit-close.response.sse", False)
    validate_responses_stream(parsed["responses_postcommit_failed"], OPENAI / "failures/responses-postcommit-failed.response.sse", True)
    cases = cancellation.get("cases", [])
    if [case.get("deliver_sse_records") for case in cases] != [2, 3]:
        fail("Cancellation fixture prefixes changed")
    for case in cases:
        source = (OPENAI / "failures" / case["source_fixture"]).resolve()
        if not source.is_file() or OPENAI.resolve() not in source.parents:
            fail("Cancellation fixture source escapes or is missing from the fixture tree")
        if case["deliver_sse_records"] >= len(parse_sse(source)):
            fail("Cancellation prefix must stop before the source stream terminal event")

    for path in OPENAI.rglob("*"):
        if not path.is_file() or path.name == "SHA256SUMS":
            continue
        text = read_text(path)
        reject_secret_patterns(path, [text])
    all_fixture_values: List[str] = []
    all_fixture_values += list(walk_values(manifest)) + list(walk_values(observations))
    all_fixture_values += list(walk_values(errors)) + list(walk_values(cancellation))
    for value in parsed.values():
        if isinstance(value, list) and value and isinstance(value[0], tuple):
            all_fixture_values += list(walk_values([record for _, record, _ in value]))
        else:
            all_fixture_values += list(walk_values(value))
    protocol_values = {
        "input", "output", "description", "city", "input_text", "output_text",
        "input_tokens", "output_tokens", "input_tokens_details", "output_tokens_details", "output_index",
        "partial", "completed", "failed", "in_progress", "tool_calls", "function_call", "message",
    }
    synthetic_values = all_fixture_values
    for value in synthetic_values:
        if re.search(r"(?i)(?:input|output|partial|description|city)", value) and "SENTINEL" not in value:
            if value not in protocol_values and not value.startswith(("response.", "failures/", "requests/")):
                fail(f"OpenAI fixtures contain a non-sentinel model-content-like value: {value!r}")
    return len(fixture_names) + 1


def main() -> int:
    try:
        opencode_count = validate_opencode()
        openai_count = validate_openai()
    except (KeyError, IndexError, TypeError) as error:
        print(f"fixture validation failed: malformed fixture structure: {error}", file=sys.stderr)
        return 1
    except ValidationError as error:
        print(f"fixture validation failed: {error}", file=sys.stderr)
        return 1
    print(f"fixture validation passed: {opencode_count} OpenCode JSON fixtures; {openai_count} OpenAI inventory files")
    print("validated UTF-8/LF/canonical JSON, checksums, all fixture privacy sentinels, config constraints, wire structure, ordering, and errors")
    return 0


if __name__ == "__main__":
    sys.exit(main())
