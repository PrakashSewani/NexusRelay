#!/usr/bin/env python3
import contextlib
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from openai import OpenAI


ROOT = Path(__file__).resolve().parents[4]
FIXTURES = ROOT / "docs/testing/fixtures/openai-sdk"
OBSERVATIONS = json.loads((FIXTURES / "requests/sdk-request-observations.json").read_text())["observations"]
RESPONSES = {
    "models_list": ("models/list.response.json", "application/json", "req_nr_models_001"),
    "chat_nonstream_tools": ("chat/nonstream-tools.response.json", "application/json", "req_nr_chat_001"),
    "chat_stream_tools_usage": ("chat/stream-tools.response.sse", "text/event-stream", "req_nr_chat_stream_001"),
    "responses_nonstream_text": ("responses/nonstream-text.response.json", "application/json", "req_nr_resp_001"),
    "responses_stream_tools_usage": ("responses/stream-tools.response.sse", "text/event-stream", "req_nr_resp_stream_001"),
    "embeddings_float": ("embeddings/float.response.json", "application/json", "req_nr_embed_001"),
}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    next_request = 0
    failure = None

    def do_GET(self):
        self.handle_scenario()

    def do_POST(self):
        self.handle_scenario()

    def handle_scenario(self):
        try:
            expected = OBSERVATIONS[type(self).next_request]
            assert self.command == expected["method"], f"{expected['id']}: method"
            assert self.path == expected["path"], f"{expected['id']}: path"
            if expected["body"] is None:
                assert int(self.headers.get("Content-Length", "0")) == 0, f"{expected['id']}: unexpected body"
                body = None
            else:
                length = int(self.headers.get("Content-Length", "0"))
                body = json.loads(self.rfile.read(length))
            assert body == expected["body"], f"{expected['id']}: sanitized request body\n{body!r}\n{expected['body']!r}"
            fixture, content_type, request_id = RESPONSES[expected["id"]]
            type(self).next_request += 1
            self.reply(fixture, content_type, request_id)
        except Exception as error:
            type(self).failure = error
            data = str(error).encode()
            self.send_response(400)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(data)))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(data)

    def reply(self, fixture, content_type, request_id):
        data = (FIXTURES / fixture).read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("x-request-id", request_id)
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *_args):
        pass


def main():
    tool = {
        "type": "function",
        "function": {
            "name": "nr_sentinel_weather",
            "description": "NR_SENTINEL_TOOL_DESCRIPTION",
            "parameters": {
                "type": "object",
                "properties": {"city": {"type": "string"}},
                "required": ["city"],
                "additionalProperties": False,
            },
            "strict": True,
        },
    }
    response_tool = {"type": "function", **tool["function"]}
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        client = OpenAI(api_key="NR_SENTINEL_FIXTURE_KEY", base_url=f"http://127.0.0.1:{server.server_port}/v1", max_retries=0)
        models = client.models.list()
        assert [model.id for model in models.data] == ["nr-chat-sentinel", "nr-responses-sentinel", "nr-embedding-sentinel"]

        chat = client.chat.completions.create(model="nr-chat-sentinel", messages=[{"role": "user", "content": "NR_SENTINEL_INPUT_CHAT_TOOL"}], tools=[tool], tool_choice="required")
        assert chat.choices[0].message.tool_calls[0].function.arguments == '{"city":"NR_SENTINEL_CITY"}'
        assert chat.usage.total_tokens == 18

        chunks = list(client.chat.completions.create(model="nr-chat-sentinel", messages=[{"role": "user", "content": "NR_SENTINEL_INPUT_CHAT_STREAM_TOOL"}], stream=True, stream_options={"include_usage": True}, tools=[tool], tool_choice="required"))
        assert chunks[-2].choices[0].finish_reason == "tool_calls"
        assert chunks[-1].choices == [] and chunks[-1].usage.total_tokens == 20

        response = client.responses.create(model="nr-responses-sentinel", input="NR_SENTINEL_INPUT_RESPONSE_TEXT")
        assert response.status == "completed" and response.output_text == "NR_SENTINEL_OUTPUT_RESPONSE_TEXT"
        assert response.usage.input_tokens == 9
        assert response.usage.input_tokens_details.cached_tokens == 0
        assert response.usage.output_tokens == 5
        assert response.usage.output_tokens_details.reasoning_tokens == 0
        assert response.usage.total_tokens == 14

        events = list(client.responses.create(model="nr-responses-sentinel", input="NR_SENTINEL_INPUT_RESPONSE_STREAM_TOOL", stream=True, tools=[response_tool], tool_choice="required"))
        assert [event.type for event in events] == [
            "response.created", "response.in_progress", "response.output_item.added",
            "response.function_call_arguments.delta", "response.output_item.done", "response.completed",
        ]
        assert [event.sequence_number for event in events] == list(range(6))
        terminal_usage = events[-1].response.usage
        assert terminal_usage.input_tokens == 13
        assert terminal_usage.input_tokens_details.cached_tokens == 0
        assert terminal_usage.output_tokens == 6
        assert terminal_usage.output_tokens_details.reasoning_tokens == 0
        assert terminal_usage.total_tokens == 19

        embeddings = client.embeddings.create(model="nr-embedding-sentinel", input=["NR_SENTINEL_EMBED_A", "NR_SENTINEL_EMBED_B"], encoding_format="float")
        assert [item.index for item in embeddings.data] == [0, 1]
        assert embeddings.data[1].embedding == [-0.375, 0.625, 0.75]
        assert embeddings.usage.prompt_tokens == 6
        assert embeddings.usage.total_tokens == 6
        assert Handler.next_request == len(OBSERVATIONS)
        if Handler.failure:
            raise Handler.failure
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
    print("python openai==2.48.0: 6 requests matched and success fixtures deserialized")


if __name__ == "__main__":
    with contextlib.suppress(KeyboardInterrupt):
        sys.exit(main())
