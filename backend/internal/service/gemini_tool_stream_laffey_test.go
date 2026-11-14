package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// These fixtures exercise the real stream handlers, including both bridge
// converters. Combining calls by name or dropping equal arguments breaks them.
func TestGeminiToolStreamLaffey_IndependentCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		for _, tc := range []struct {
			name  string
			parts []string
			want  []string
		}{
			{"distinct IDs", []string{
				`{"functionCall":{"id":"a","name":"Read","args":{"path":"a.txt"}}}`,
				`{"functionCall":{"id":"b","name":"Read","args":{"path":"b.txt"}}}`,
				`{"functionCall":{"id":"c","name":"Read","args":{"path":"c.txt"}}}`,
			}, []string{`{"path":"a.txt"}`, `{"path":"b.txt"}`, `{"path":"c.txt"}`}},
			{"equal arguments", []string{
				`{"functionCall":{"id":"a","name":"Read","args":{"path":"same"}}}`,
				`{"functionCall":{"id":"b","name":"Read","args":{"path":"same"}}}`,
			}, []string{`{"path":"same"}`, `{"path":"same"}`}},
			{"without IDs", []string{
				`{"functionCall":{"name":"Read","args":{"path":"same"}}}`,
				`{"functionCall":{"name":"Read","args":{"path":"same"}}}`,
			}, []string{`{"path":"same"}`, `{"path":"same"}`}},
			{"same frame", []string{
				`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}}},{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
			}, []string{`{"path":"a"}`, `{"path":"b"}`}},
			{"large integer", []string{
				`{"functionCall":{"id":"a","name":"Read","args":{"number":9007199254740993,"text":"}{"}}}`,
			}, []string{`{"number":9007199254740993,"text":"}{"}`}},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				out, err := runGeminiLaffeyStream(t, protocol, geminiLaffeySSE(tc.parts...), nil)
				require.NoError(t, err)
				calls := assembleGeminiLaffeyCalls(t, protocol, out)
				require.Len(t, calls, len(tc.want), out)
				ids := make(map[string]bool)
				for i, call := range calls {
					require.NotEmpty(t, call.ID)
					require.False(t, ids[call.ID], "duplicate ID")
					ids[call.ID] = true
					require.Equal(t, "Read", call.Function.Name)
					var got, want any
					decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
					decoder.UseNumber()
					require.NoError(t, decoder.Decode(&got), call.Function.Arguments)
					require.ErrorIs(t, decoder.Decode(new(any)), io.EOF)
					decoder = json.NewDecoder(strings.NewReader(tc.want[i]))
					decoder.UseNumber()
					require.NoError(t, decoder.Decode(&want))
					require.Equal(t, want, got)
				}
			})
		}
	}
}

func geminiLaffeySSE(parts ...string) string {
	var body strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&body, "data: {\"candidates\":[{\"content\":{\"parts\":[%s]}}]}\n\n", part)
	}
	_, _ = body.WriteString("data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":7,\"candidatesTokenCount\":3}}\n\n")
	return body.String()
}

func runGeminiLaffeyStream(t *testing.T, protocol, input string, cfg *config.Config) (string, error) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(input))}
	svc := &GeminiMessagesCompatService{cfg: cfg}
	var err error
	if protocol == "chat" {
		_, err = svc.handleChatCompletionsStreamingResponseFromGemini(c, resp, time.Now(), "gemini-4-hydrogen-high", false, true)
	} else {
		_, err = svc.handleStreamingResponse(c, resp, time.Now(), "gemini-4-hydrogen-high")
	}
	return w.Body.String(), err
}

func assembleGeminiLaffeyCalls(t *testing.T, protocol, output string) []apicompat.ChatToolCall {
	t.Helper()
	var calls []apicompat.ChatToolCall
	blocks := make(map[int]int)
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		data := []byte(strings.TrimPrefix(line, "data: "))
		if protocol == "chat" {
			var chunk apicompat.ChatCompletionsChunk
			require.NoError(t, json.Unmarshal(data, &chunk))
			for _, choice := range chunk.Choices {
				for _, call := range choice.Delta.ToolCalls {
					require.NotNil(t, call.Index)
					index := *call.Index
					if call.ID != "" {
						require.Equal(t, len(calls), index)
						calls = append(calls, apicompat.ChatToolCall{ID: call.ID, Function: apicompat.ChatFunctionCall{Name: call.Function.Name}})
					}
					require.Less(t, index, len(calls))
					calls[index].Function.Arguments += call.Function.Arguments
				}
			}
		} else {
			var event apicompat.AnthropicStreamEvent
			require.NoError(t, json.Unmarshal(data, &event))
			if event.Type == "content_block_start" && event.ContentBlock.Type == "tool_use" {
				require.NotNil(t, event.Index)
				blocks[*event.Index] = len(calls)
				calls = append(calls, apicompat.ChatToolCall{ID: event.ContentBlock.ID, Function: apicompat.ChatFunctionCall{Name: event.ContentBlock.Name}})
			}
			if event.Type == "content_block_delta" && event.Delta.Type == "input_json_delta" {
				require.NotNil(t, event.Index)
				index, ok := blocks[*event.Index]
				require.True(t, ok)
				calls[index].Function.Arguments += event.Delta.PartialJSON
			}
		}
	}
	return calls
}

func TestGeminiToolStreamLaffey_ProtocolErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		for _, tc := range []struct {
			name, input string
			cfg         *config.Config
		}{
			{"incomplete args", geminiLaffeySSE(`{"functionCall":{"id":"a","name":"Read","args":"{\"path\":"}}`), nil},
			{"unsupported partial args", geminiLaffeySSE(`{"functionCall":{"id":"a","name":"Read","partialArgs":[{}]}}`), nil},
			{"invalid frame", "data: {broken\n\n", nil},
			{"upstream error", "data: {\"error\":{\"message\":\"synthetic upstream error\"}}\n\n", nil},
			{"unfinished stream", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"Read\",\"args\":{}}}]}}]}\n\n", nil},
			{"byte limit", geminiLaffeySSE(`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}}}`), &config.Config{Gateway: config.GatewayConfig{UpstreamResponseReadMaxBytes: 16}}},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				out, err := runGeminiLaffeyStream(t, protocol, tc.input, tc.cfg)
				require.Error(t, err, out)
				require.Contains(t, out, `"type":"upstream_error"`)
				require.NotContains(t, out, `"finish_reason":"tool_calls"`)
				require.NotContains(t, out, `"type":"message_stop"`)
				require.NotContains(t, out, "data: [DONE]")
			})
		}
	}
}

func TestGeminiToolStreamLaffey_LengthAndDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		input := strings.ReplaceAll(geminiLaffeySSE(`{"functionCall":{"name":"Read","args":{}}}`), "STOP", "MAX_TOKENS")
		out, err := runGeminiLaffeyStream(t, protocol, input, nil)
		require.NoError(t, err)
		if protocol == "chat" {
			require.Contains(t, out, `"finish_reason":"length"`)
		} else {
			require.Contains(t, out, `"stop_reason":"max_tokens"`)
		}
	}
}

func TestGeminiToolStreamLaffey_FragmentedOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		out, err := runGeminiLaffeyStream(t, protocol, geminiLaffeySSE(
			`{"functionCall":{"id":"a","name":"Read","args":"{\"path\":\""}}`,
			`{"text":"after-a"}`,
			`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
			`{"functionCall":{"id":"a","args":"a\"}"}}`,
		), nil)
		require.NoError(t, err)
		calls := assembleGeminiLaffeyCalls(t, protocol, out)
		require.Len(t, calls, 2)
		require.JSONEq(t, `{"path":"a"}`, calls[0].Function.Arguments)
		require.JSONEq(t, `{"path":"b"}`, calls[1].Function.Arguments)
		require.Less(t, strings.Index(out, calls[0].ID), strings.Index(out, "after-a"))
		require.Less(t, strings.Index(out, "after-a"), strings.Index(out, calls[1].ID))
	}
}

func TestGeminiToolStreamLaffey_ResultTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			out, err := runGeminiLaffeyStream(t, protocol, geminiLaffeySSE(
				`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}}}`,
				`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
				`{"functionCall":{"id":"c","name":"Read","args":{"path":"c"}}}`,
			), nil)
			require.NoError(t, err)
			calls := assembleGeminiLaffeyCalls(t, protocol, out)
			require.Len(t, calls, 3)
			messages := []any{map[string]any{"role": "user", "content": "Read a, b, c"}}
			if protocol == "chat" {
				messages = append(messages, map[string]any{"role": "assistant", "tool_calls": calls})
				for i, call := range calls {
					messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call.ID, "content": fmt.Sprintf("result-%d", i)})
				}
			} else {
				var toolUses, results []any
				for i, call := range calls {
					toolUses = append(toolUses, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": json.RawMessage(call.Function.Arguments)})
					results = append(results, map[string]any{"type": "tool_result", "tool_use_id": call.ID, "content": fmt.Sprintf("result-%d", i)})
				}
				messages = append(messages, map[string]any{"role": "assistant", "content": toolUses}, map[string]any{"role": "user", "content": results})
			}
			body, err := json.Marshal(map[string]any{"model": "gemini-4-hydrogen-high", "messages": messages, "max_tokens": 128})
			require.NoError(t, err)
			upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"OK"}]},"finishReason":"STOP"}]}`))}}
			svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
			account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
			if protocol == "chat" {
				_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
			} else {
				_, err = svc.Forward(context.Background(), c, account, body)
			}
			require.NoError(t, err)
			require.Contains(t, w.Body.String(), "OK")
			sent, err := io.ReadAll(upstream.lastReq.Body)
			require.NoError(t, err)
			var request map[string]any
			require.NoError(t, json.Unmarshal(sent, &request))
			var gotCalls, gotResults []string
			contents, ok := request["contents"].([]any)
			require.True(t, ok)
			for _, content := range contents {
				parts, ok := laffeyTestObject(t, content)["parts"].([]any)
				require.True(t, ok)
				for _, p := range parts {
					part := laffeyTestObject(t, p)
					if call, ok := part["functionCall"].(map[string]any); ok {
						path, ok := laffeyTestObject(t, call["args"])["path"].(string)
						require.True(t, ok)
						gotCalls = append(gotCalls, path)
					}
					if result, ok := part["functionResponse"].(map[string]any); ok {
						require.Equal(t, "Read", result["name"])
						text, ok := laffeyTestObject(t, result["response"])["content"].(string)
						require.True(t, ok)
						gotResults = append(gotResults, text)
					}
				}
			}
			require.Equal(t, []string{"a", "b", "c"}, gotCalls)
			require.Equal(t, []string{"result-0", "result-1", "result-2"}, gotResults)
		})
	}
}

func TestGeminiToolStreamLaffey_ProtocolErrorRetainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages"} {
		input := `data: {"candidates":[{"content":{"parts":[{"text":"before"}]}}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}` + "\n\n" +
			`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"Read","args":"{"}}]}}]}` + "\n\n"
		upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(input))}}
		svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
		body := []byte(`{"model":"gemini-4-hydrogen-high","stream":true,"messages":[{"role":"user","content":"read"}]}`)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		var result *ForwardResult
		var err error
		if protocol == "chat" {
			result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
		} else {
			result, err = svc.Forward(context.Background(), c, account, body)
		}
		require.Error(t, err)
		require.NotNil(t, result, "%s discarded billable usage", protocol)
		require.Equal(t, 7, result.Usage.InputTokens)
		require.Equal(t, 3, result.Usage.OutputTokens)
	}
}

func TestGeminiToolStreamLaffey_ErrorFrameUsageAndImageCharge(t *testing.T) {
	for _, model := range []string{"gemini-4-hydrogen-high", "gemini-3-pro-image"} {
		for _, protocol := range []string{"chat", "messages"} {
			input := `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"Read","args":[]}}]}}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}` + "\n\n"
			upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(input))}}
			svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
			body := []byte(fmt.Sprintf(`{"model":%q,"stream":true,"messages":[{"role":"user","content":"read"}]}`, model))
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Set("gemini_image_generation_allowed", true)
			var result *ForwardResult
			var err error
			if protocol == "chat" {
				result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
			} else {
				result, err = svc.Forward(context.Background(), c, account, body)
			}
			require.Error(t, err)
			require.NotNil(t, result, "%s/%s", protocol, model)
			require.Equal(t, 7, result.Usage.InputTokens, "%s/%s", protocol, model)
			require.Equal(t, 3, result.Usage.OutputTokens)
			require.Zero(t, result.ImageCount, "failed conversion must not manufacture image charges")
		}
	}
}
