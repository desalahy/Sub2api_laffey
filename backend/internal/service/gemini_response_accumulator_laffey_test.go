package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiResponseAccumulatorLaffey_AllParts(t *testing.T) {
	input := geminiLaffeySSE(
		`{"text":"hel"}`,
		`{"text":"hello"}`,
		`{"thought":true,"text":"thinking","thoughtSignature":"thought-signature"}`,
		`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}},"thoughtSignature":"call-signature"}`,
		`{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`,
		`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
	)
	result, usage, err := collectGeminiSSE(nil, strings.NewReader(input), false)
	require.NoError(t, err)
	parts := extractGeminiParts(result)
	require.Len(t, parts, 5)
	require.Equal(t, "hello", parts[0]["text"])
	require.Equal(t, true, parts[1]["thought"])
	require.Equal(t, "thought-signature", parts[1]["thoughtSignature"])
	require.Equal(t, "call-signature", parts[2]["thoughtSignature"])
	firstCall, ok := parts[2]["functionCall"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "a", firstCall["id"])
	require.Contains(t, parts[3], "inlineData")
	lastCall, ok := parts[4]["functionCall"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "b", lastCall["id"])
	require.Equal(t, "STOP", extractGeminiFinishReason(result))
	require.Equal(t, 7, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
}

func TestGeminiResponseAccumulatorLaffey_NativeOAuth(t *testing.T) {
	input := geminiLaffeySSE(
		`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}},"thoughtSignature":"sig"}`,
		`{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`,
		`{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`,
		`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
	)
	var wrapped strings.Builder
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "data: ") {
			_, _ = wrapped.WriteString("data: {\"response\":" + strings.TrimPrefix(line, "data: ") + "}\n\n")
		}
	}
	upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(wrapped.String()))}}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, tokenProvider: &GeminiTokenProvider{}, cfg: &config.Config{}}
	account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token", "project_id": "test-project"}, Concurrency: 1}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-4-hydrogen-high:generateContent", nil)
	forward, err := svc.ForwardNative(context.Background(), c, account, "gemini-4-hydrogen-high", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"Read a and b"}]}]}`))
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	parts := extractGeminiParts(result)
	require.Len(t, parts, 3)
	require.Equal(t, "sig", parts[0]["thoughtSignature"])
	require.Equal(t, "STOP", extractGeminiFinishReason(result))
	require.Equal(t, 1, forward.ImageCount, "cumulative image must be billed once")
}

func TestGeminiResponseAccumulatorLaffey_MultipleCandidates(t *testing.T) {
	input := `data: {"candidates":[{"index":0,"content":{"parts":[{"text":"a"}]},"finishReason":"STOP"},{"index":1,"content":{"parts":[{"text":"b"}]},"finishReason":"STOP"}]}` + "\n\n"
	result, _, err := collectGeminiSSE(nil, strings.NewReader(input), false)
	require.NoError(t, err)
	candidates, ok := result["candidates"].([]any)
	require.True(t, ok)
	require.Len(t, candidates, 2)
	secondParts := extractGeminiParts(map[string]any{"candidates": []any{candidates[1]}})
	require.Equal(t, "b", secondParts[0]["text"])
}

func TestGeminiResponseAccumulatorLaffey_CumulativeImages(t *testing.T) {
	input := geminiLaffeySSE(`{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`, `{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`)
	result, _, err := collectGeminiSSE(nil, strings.NewReader(input), false)
	require.NoError(t, err)
	require.Len(t, extractGeminiParts(result), 1)
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	require.Equal(t, 1, countGeminiImageOutputs(raw))
}

func TestGeminiResponseAccumulatorLaffey_IncompleteTerminal(t *testing.T) {
	call := `data: {"candidates":[{"content":{"parts":[{"functionCall":{"id":"a","name":"Read","args":{}}}]}}]}` + "\n\n"
	for _, tail := range []string{"", "data: {broken\n\n"} {
		_, _, err := collectGeminiSSE(nil, strings.NewReader(call+tail), false)
		require.Error(t, err)
	}
	_, _, err := collectGeminiSSE(nil, strings.NewReader(call+"data: [DONE]\n\n"), false)
	require.NoError(t, err)
}

func TestGeminiResponseAccumulatorLaffey_NonstreamSemantics(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"thought":true,"text":"reasoning"},{"text":"answer"},{"functionCall":{"name":"Read","args":{"n":9007199254740993}}}]},"finishReason":"MAX_TOKENS"}]}`)
	response, err := decodeGeminiObjectLaffey(raw)
	require.NoError(t, err)
	chat, _, err := geminiResponseToChatCompletions(response, "gemini-4-hydrogen-high", raw, nil)
	require.NoError(t, err)
	require.Equal(t, "length", chat.Choices[0].FinishReason)
	require.NotContains(t, string(chat.Choices[0].Message.Content), "reasoning")
	require.Equal(t, "reasoning", chat.Choices[0].Message.ReasoningContent)
	require.Contains(t, chat.Choices[0].Message.ToolCalls[0].Function.Arguments, "9007199254740993")
}

func TestGeminiResponseAccumulatorLaffey_UpstreamErrorNotToolSuccess(t *testing.T) {
	input := geminiLaffeySSE(`{"functionCall":{"id":"a","name":"Read","args":{}}}`) + "data: {\"error\":{\"message\":\"upstream failure\"}}\n\n"
	response, usage, err := collectGeminiSSE(nil, strings.NewReader(input), false)
	require.NoError(t, err) // Native API preserves the actual error envelope.
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	chat, _, err := geminiResponseToChatCompletions(response, "gemini-4-hydrogen-high", raw, usage)
	require.Error(t, err)
	require.Nil(t, chat)
}

func TestGeminiResponseAccumulatorLaffey_DirectNonstreamInteger(t *testing.T) {
	raw := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"Read","args":{"n":9007199254740993}}}]},"finishReason":"STOP"}]}`
	for _, protocol := range []string{"chat", "messages"} {
		upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(raw))}}
		svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
		body := []byte(`{"model":"gemini-4-hydrogen-high","messages":[{"role":"user","content":"read"}]}`)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		var err error
		if protocol == "chat" {
			_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
		} else {
			_, err = svc.Forward(context.Background(), c, account, body)
		}
		require.NoError(t, err)
		require.Contains(t, w.Body.String(), "9007199254740993")
	}
}
