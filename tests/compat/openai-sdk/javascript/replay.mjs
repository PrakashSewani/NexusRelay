import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import OpenAI from "openai";

const here = dirname(fileURLToPath(import.meta.url));
const fixtures = resolve(here, "../../../../docs/testing/fixtures/openai-sdk");
const observations = JSON.parse(await readFile(join(fixtures, "requests/sdk-request-observations.json"))).observations;
const responses = {
  models_list: ["models/list.response.json", "application/json", "req_nr_models_001"],
  chat_nonstream_tools: ["chat/nonstream-tools.response.json", "application/json", "req_nr_chat_001"],
  chat_stream_tools_usage: ["chat/stream-tools.response.sse", "text/event-stream", "req_nr_chat_stream_001"],
  responses_nonstream_text: ["responses/nonstream-text.response.json", "application/json", "req_nr_resp_001"],
  responses_stream_tools_usage: ["responses/stream-tools.response.sse", "text/event-stream", "req_nr_resp_stream_001"],
  embeddings_float: ["embeddings/float.response.json", "application/json", "req_nr_embed_001"],
};
let nextRequest = 0;

const server = createServer(async (request, response) => {
  try {
    const expected = observations[nextRequest];
    assert.ok(expected, `unexpected extra request: ${request.method} ${request.url}`);
    assert.equal(request.method, expected.method, `${expected.id}: method`);
    assert.equal(request.url, expected.path, `${expected.id}: path`);
    let body = null;
    if (expected.body !== null) {
      let raw = "";
      for await (const chunk of request) raw += chunk;
      body = JSON.parse(raw);
    }
    assert.deepEqual(body, expected.body, `${expected.id}: sanitized request body`);
    const [fixture, contentType, requestID] = responses[expected.id];
    const data = await readFile(join(fixtures, fixture));
    nextRequest += 1;
    response.writeHead(200, { "content-type": contentType, "content-length": data.length, "x-request-id": requestID });
    response.end(data);
  } catch (error) {
    response.writeHead(400, { "content-type": "text/plain", connection: "close" });
    response.end(String(error));
    server.emit("replayError", error);
  }
});

let replayError;
server.once("replayError", (error) => { replayError = error; });
await new Promise((resolveListen) => server.listen(0, "127.0.0.1", resolveListen));
const tool = {
  type: "function",
  function: {
    name: "nr_sentinel_weather",
    description: "NR_SENTINEL_TOOL_DESCRIPTION",
    parameters: {
      type: "object",
      properties: { city: { type: "string" } },
      required: ["city"],
      additionalProperties: false,
    },
    strict: true,
  },
};
const responseTool = { type: "function", ...tool.function };

try {
  const { port } = server.address();
  const client = new OpenAI({ apiKey: "NR_SENTINEL_FIXTURE_KEY", baseURL: `http://127.0.0.1:${port}/v1`, maxRetries: 0 });
  const models = await client.models.list();
  assert.deepEqual(models.data.map((model) => model.id), ["nr-chat-sentinel", "nr-responses-sentinel", "nr-embedding-sentinel"]);

  const chat = await client.chat.completions.create({ model: "nr-chat-sentinel", messages: [{ role: "user", content: "NR_SENTINEL_INPUT_CHAT_TOOL" }], tools: [tool], tool_choice: "required" });
  assert.equal(chat.choices[0].message.tool_calls[0].function.arguments, '{"city":"NR_SENTINEL_CITY"}');
  assert.equal(chat.usage.total_tokens, 18);

  const chatStream = await client.chat.completions.create({ model: "nr-chat-sentinel", messages: [{ role: "user", content: "NR_SENTINEL_INPUT_CHAT_STREAM_TOOL" }], stream: true, stream_options: { include_usage: true }, tools: [tool], tool_choice: "required" });
  const chunks = [];
  for await (const chunk of chatStream) chunks.push(chunk);
  assert.equal(chunks.at(-2).choices[0].finish_reason, "tool_calls");
  assert.equal(chunks.at(-1).usage.total_tokens, 20);

  const completed = await client.responses.create({ model: "nr-responses-sentinel", input: "NR_SENTINEL_INPUT_RESPONSE_TEXT" });
  assert.equal(completed.status, "completed");
  assert.equal(completed.output_text, "NR_SENTINEL_OUTPUT_RESPONSE_TEXT");
  assert.deepEqual(completed.usage, { input_tokens: 9, input_tokens_details: { cached_tokens: 0 }, output_tokens: 5, output_tokens_details: { reasoning_tokens: 0 }, total_tokens: 14 });

  const responseStream = await client.responses.create({ model: "nr-responses-sentinel", input: "NR_SENTINEL_INPUT_RESPONSE_STREAM_TOOL", stream: true, tools: [responseTool], tool_choice: "required" });
  const events = [];
  for await (const event of responseStream) events.push(event);
  assert.deepEqual(events.map((event) => event.type), ["response.created", "response.in_progress", "response.output_item.added", "response.function_call_arguments.delta", "response.output_item.done", "response.completed"]);
  assert.deepEqual(events.map((event) => event.sequence_number), [0, 1, 2, 3, 4, 5]);
  assert.deepEqual(events.at(-1).response.usage, { input_tokens: 13, input_tokens_details: { cached_tokens: 0 }, output_tokens: 6, output_tokens_details: { reasoning_tokens: 0 }, total_tokens: 19 });

  const embeddings = await client.embeddings.create({ model: "nr-embedding-sentinel", input: ["NR_SENTINEL_EMBED_A", "NR_SENTINEL_EMBED_B"], encoding_format: "float" });
  assert.deepEqual(embeddings.data.map((item) => item.index), [0, 1]);
  assert.deepEqual(embeddings.data[1].embedding, [-0.375, 0.625, 0.75]);
  assert.deepEqual(embeddings.usage, { prompt_tokens: 6, total_tokens: 6 });
  assert.equal(nextRequest, observations.length);
  if (replayError) throw replayError;
} finally {
  await new Promise((resolveClose) => server.close(resolveClose));
}

console.log("javascript openai@6.49.0: 6 requests matched and success fixtures deserialized");
