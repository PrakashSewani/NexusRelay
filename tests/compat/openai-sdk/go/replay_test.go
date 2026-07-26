package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type observation struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Body   map[string]any `json:"body"`
}

type replayServer struct {
	t            *testing.T
	fixtures     string
	observations []observation
	mu           sync.Mutex
	next         int
}

func fixtureRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../docs/testing/fixtures/openai-sdk"))
}

func newReplayServer(t *testing.T) (*replayServer, *httptest.Server) {
	t.Helper()
	root := fixtureRoot()
	data, err := os.ReadFile(filepath.Join(root, "requests/sdk-request-observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Observations []observation `json:"observations"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	handler := &replayServer{t: t, fixtures: root, observations: document.Observations}
	return handler, httptest.NewServer(handler)
}

func (s *replayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next >= len(s.observations) {
		http.Error(w, "unexpected extra request", http.StatusBadRequest)
		s.t.Errorf("unexpected extra request: %s %s", r.Method, r.URL.Path)
		return
	}
	expected := s.observations[s.next]
	if r.Method != expected.Method || r.URL.Path != expected.Path || r.URL.RawQuery != "" {
		http.Error(w, "unexpected request", http.StatusBadRequest)
		s.t.Errorf("request %d: got %s %s, want %s %s", s.next+1, r.Method, r.URL.RequestURI(), expected.Method, expected.Path)
		return
	}
	var actual map[string]any
	if expected.Body == nil {
		if r.Body != nil {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				s.t.Error(err)
				return
			}
			if len(data) != 0 {
				s.t.Errorf("%s: expected no body, got %q", expected.ID, data)
				return
			}
		}
	} else if err := json.NewDecoder(r.Body).Decode(&actual); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		s.t.Errorf("%s: invalid request JSON: %v", expected.ID, err)
		return
	} else if !reflect.DeepEqual(actual, expected.Body) {
		http.Error(w, "request body mismatch", http.StatusBadRequest)
		s.t.Errorf("%s request body mismatch\n got: %#v\nwant: %#v", expected.ID, actual, expected.Body)
		return
	}
	fixture, contentType, requestID := responseFor(expected.ID)
	data, err := os.ReadFile(filepath.Join(s.fixtures, fixture))
	if err != nil {
		http.Error(w, "fixture unavailable", http.StatusInternalServerError)
		s.t.Error(err)
		return
	}
	s.next++
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.Header().Set("x-request-id", requestID)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		s.t.Error(err)
	}
}

func responseFor(id string) (string, string, string) {
	switch id {
	case "models_list":
		return "models/list.response.json", "application/json", "req_nr_models_001"
	case "chat_nonstream_tools":
		return "chat/nonstream-tools.response.json", "application/json", "req_nr_chat_001"
	case "chat_stream_tools_usage":
		return "chat/stream-tools.response.sse", "text/event-stream", "req_nr_chat_stream_001"
	case "responses_nonstream_text":
		return "responses/nonstream-text.response.json", "application/json", "req_nr_resp_001"
	case "responses_stream_tools_usage":
		return "responses/stream-tools.response.sse", "text/event-stream", "req_nr_resp_stream_001"
	case "embeddings_float":
		return "embeddings/float.response.json", "application/json", "req_nr_embed_001"
	default:
		panic("unknown observation: " + id)
	}
}

func toolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
		},
		"required":             []string{"city"},
		"additionalProperties": false,
	}
}

func chatParams(input string, stream bool) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:    "nr-chat-sentinel",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(input)},
		Tools: []openai.ChatCompletionToolUnionParam{openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        "nr_sentinel_weather",
			Description: param.NewOpt("NR_SENTINEL_TOOL_DESCRIPTION"),
			Parameters:  toolSchema(),
			Strict:      param.NewOpt(true),
		})},
		ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: param.NewOpt("required")},
	}
	if stream {
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: param.NewOpt(true)}
	}
	return params
}

func responseParams(input string, withTool bool) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: "nr-responses-sentinel",
		Input: responses.ResponseNewParamsInputUnion{OfString: param.NewOpt(input)},
	}
	if withTool {
		tool := responses.ToolParamOfFunction("nr_sentinel_weather", toolSchema(), true)
		tool.OfFunction.Description = param.NewOpt("NR_SENTINEL_TOOL_DESCRIPTION")
		params.Tools = []responses.ToolUnionParam{tool}
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsRequired)}
	}
	return params
}

func TestSuccessFixturesReplayThroughSDK(t *testing.T) {
	handler, server := newReplayServer(t)
	defer server.Close()
	client := openai.NewClient(
		option.WithAPIKey("NR_SENTINEL_FIXTURE_KEY"),
		option.WithBaseURL(server.URL+"/v1"),
		option.WithMaxRetries(0),
	)
	ctx := context.Background()

	models, err := client.Models.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Data) != 3 || models.Data[0].ID != "nr-chat-sentinel" {
		t.Fatalf("unexpected models: %#v", models.Data)
	}

	chat, err := client.Chat.Completions.New(ctx, chatParams("NR_SENTINEL_INPUT_CHAT_TOOL", false))
	if err != nil {
		t.Fatal(err)
	}
	if chat.Choices[0].Message.ToolCalls[0].Function.Arguments != `{"city":"NR_SENTINEL_CITY"}` || chat.Usage.TotalTokens != 18 {
		t.Fatalf("unexpected chat fixture: %#v", chat)
	}

	chatStream := client.Chat.Completions.NewStreaming(ctx, chatParams("NR_SENTINEL_INPUT_CHAT_STREAM_TOOL", true))
	var chatChunks []openai.ChatCompletionChunk
	for chatStream.Next() {
		chatChunks = append(chatChunks, chatStream.Current())
	}
	if err := chatStream.Err(); err != nil {
		t.Fatal(err)
	}
	if len(chatChunks) < 2 || chatChunks[len(chatChunks)-2].Choices[0].FinishReason != "tool_calls" || chatChunks[len(chatChunks)-1].Usage.TotalTokens != 20 {
		t.Fatalf("unexpected chat stream terminal records: %#v", chatChunks)
	}

	response, err := client.Responses.New(ctx, responseParams("NR_SENTINEL_INPUT_RESPONSE_TEXT", false))
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "completed" || response.Output[0].AsMessage().Content[0].AsOutputText().Text != "NR_SENTINEL_OUTPUT_RESPONSE_TEXT" {
		t.Fatalf("unexpected response fixture: %#v", response)
	}
	if response.Usage.InputTokens != 9 || response.Usage.InputTokensDetails.CachedTokens != 0 || response.Usage.OutputTokens != 5 || response.Usage.OutputTokensDetails.ReasoningTokens != 0 || response.Usage.TotalTokens != 14 {
		t.Fatalf("unexpected response usage: %#v", response.Usage)
	}

	responseStream := client.Responses.NewStreaming(ctx, responseParams("NR_SENTINEL_INPUT_RESPONSE_STREAM_TOOL", true))
	var eventTypes []string
	var terminalUsage responses.ResponseUsage
	for responseStream.Next() {
		event := responseStream.Current()
		if event.SequenceNumber != int64(len(eventTypes)) {
			t.Fatalf("unexpected sequence number: %d", event.SequenceNumber)
		}
		eventTypes = append(eventTypes, event.Type)
		if event.Type == "response.completed" {
			terminalUsage = event.AsResponseCompleted().Response.Usage
		}
	}
	if err := responseStream.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(eventTypes, ",") != "response.created,response.in_progress,response.output_item.added,response.function_call_arguments.delta,response.output_item.done,response.completed" {
		t.Fatalf("unexpected response events: %v", eventTypes)
	}
	if terminalUsage.InputTokens != 13 || terminalUsage.InputTokensDetails.CachedTokens != 0 || terminalUsage.OutputTokens != 6 || terminalUsage.OutputTokensDetails.ReasoningTokens != 0 || terminalUsage.TotalTokens != 19 {
		t.Fatalf("unexpected terminal response usage: %#v", terminalUsage)
	}

	embeddings, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model:          "nr-embedding-sentinel",
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{"NR_SENTINEL_EMBED_A", "NR_SENTINEL_EMBED_B"}},
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings.Data) != 2 || embeddings.Data[1].Index != 1 || !reflect.DeepEqual(embeddings.Data[1].Embedding, []float64{-0.375, 0.625, 0.75}) {
		t.Fatalf("unexpected embeddings fixture: %#v", embeddings)
	}
	if embeddings.Usage.PromptTokens != 6 || embeddings.Usage.TotalTokens != 6 {
		t.Fatalf("unexpected embeddings usage: %#v", embeddings.Usage)
	}
	if handler.next != len(handler.observations) {
		t.Fatalf("served %d of %d expected requests", handler.next, len(handler.observations))
	}
}
