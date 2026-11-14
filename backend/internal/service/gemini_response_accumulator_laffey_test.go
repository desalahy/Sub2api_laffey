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
	require.Equal(t, "a", parts[2]["functionCall"].(map[string]any)["id"])
	require.Contains(t, parts[3], "inlineData")
	require.Equal(t, "b", parts[4]["functionCall"].(map[string]any)["id"])
	require.Equal(t, "STOP", extractGeminiFinishReason(result))
	require.Equal(t, 7, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
}

func TestGeminiResponseAccumulatorLaffey_NativeOAuth(t *testing.T) {
	input := geminiLaffeySSE(
		`{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}},"thoughtSignature":"sig"}`,
		`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
	)
	var wrapped strings.Builder
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "data: ") {
			wrapped.WriteString("data: {\"response\":" + strings.TrimPrefix(line, "data: ") + "}\n\n")
		}
	}
	upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(wrapped.String()))}}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, tokenProvider: &GeminiTokenProvider{}, cfg: &config.Config{}}
	account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token", "project_id": "test-project"}, Concurrency: 1}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-4-hydrogen-high:generateContent", nil)
	_, err := svc.ForwardNative(context.Background(), c, account, "gemini-4-hydrogen-high", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"Read a and b"}]}]}`))
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	parts := extractGeminiParts(result)
	require.Len(t, parts, 2)
	require.Equal(t, "sig", parts[0]["thoughtSignature"])
	require.Equal(t, "STOP", extractGeminiFinishReason(result))
}
