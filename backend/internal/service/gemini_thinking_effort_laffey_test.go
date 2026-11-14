//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// This exercises the real adapters: losing the original Chat effort in the
// intermediate Anthropic bridge must change the captured upstream request.
func TestGeminiThinkingEffortLaffey_ForwardPreservesExplicitLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name     string
		model    string
		mapped   string
		effort   string
		want     string
		disabled bool
	}{
		{name: "hydrogen xhigh", model: "gemini-4-hydrogen", effort: "xhigh", want: "xhigh"},
		{name: "hydrogen max distinct", model: "gemini-4-hydrogen", effort: "max", want: "max"},
		{name: "hydrogen high", model: "gemini-4-hydrogen", effort: "high", want: "high"},
		{name: "carbon high", model: "gemini-4-carbon", effort: "high", want: "high"},
		{name: "carbon medium", model: "gemini-4-carbon", effort: "medium", want: "medium"},
		{name: "carbon max", model: "gemini-4-carbon", effort: "max", want: "max"},
		{name: "carbon explicit unsupported tier reaches upstream validation", model: "gemini-4-carbon", effort: "xhigh", want: "xhigh"},
		{name: "mapped hydrogen", model: "client-model", mapped: "gemini-4-hydrogen", effort: "xhigh", want: "xhigh"},
		{name: "mapped legacy", model: "gemini-4-hydrogen", mapped: "gemini-3-pro-preview", effort: "xhigh", want: "high"},
		{name: "legacy flash xhigh", model: "gemini-3.8-flash", effort: "xhigh", want: "high"},
		{name: "legacy pro medium", model: "gemini-3-pro-preview", effort: "medium", want: "high"},
		{name: "legacy flash medium", model: "gemini-3.8-flash", effort: "medium", want: "medium"},
		{name: "suffix tier", model: "gemini-4-hydrogen-high", effort: "xhigh"},
		{name: "omitted", model: "gemini-4-hydrogen"},
		{name: "explicit disabled", model: "gemini-4-hydrogen", effort: "xhigh", want: "low", disabled: true},
	}
	for _, protocol := range []string{"chat", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, tc := range cases {
				if protocol == "chat" && tc.disabled {
					continue
				}
				mode := "nonstream"
				if stream {
					mode = "stream"
				}
				t.Run(protocol+"/"+mode+"/"+tc.name, func(t *testing.T) {
					response := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}`
					contentType := "application/json"
					if stream {
						response = "data: " + response + "\n\n"
						contentType = "text/event-stream"
					}
					upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{contentType}},
						Body:       io.NopCloser(strings.NewReader(response)),
					}}}
					svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
					account := geminiCompatThinkingTestAccount()
					if tc.mapped != "" {
						account.Credentials["model_mapping"] = map[string]any{tc.model: tc.mapped}
					}
					payload := map[string]any{
						"model": tc.model, "stream": stream, "max_tokens": 256,
						"messages": []any{map[string]any{"role": "user", "content": "hi"}},
					}
					if tc.effort != "" {
						if protocol == "chat" {
							payload["reasoning_effort"] = tc.effort
						} else {
							payload["output_config"] = map[string]any{"effort": tc.effort}
						}
					}
					if tc.disabled {
						payload["thinking"] = map[string]any{"type": "disabled"}
					}
					body, err := json.Marshal(payload)
					require.NoError(t, err)
					c, recorder := newAntigravityCompatContext(http.MethodPost, "/v1/messages", body)
					var result *ForwardResult
					if protocol == "chat" {
						result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
					} else {
						result, err = svc.Forward(context.Background(), c, account, body)
					}
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Len(t, upstream.requestBodies, 1)
					level := gjson.GetBytes(upstream.requestBodies[0], "generationConfig.thinkingConfig.thinkingLevel")
					require.Equal(t, tc.want, level.String(), "client effort must survive all intermediate conversions")
					if tc.want == "" {
						require.False(t, level.Exists())
						require.Nil(t, result.ReasoningEffort)
					} else {
						require.Equal(t, tc.want, optionalStringValue(result.ReasoningEffort), "usage must record the actual upstream level")
					}
				})
			}
		}
	}
}

func TestGeminiThinkingEffortLaffey_SignatureRetryPreservesXHigh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":400,"message":"Function call is missing a thought_signature","status":"INVALID_ARGUMENT"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: " + `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}` + "\n\n")),
		},
	}}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	body := []byte(`{"model":"gemini-4-hydrogen","stream":true,"max_tokens":256,"thinking":{"type":"adaptive"},"output_config":{"effort":"xhigh"},"messages":[{"role":"user","content":"hello"}]}`)
	c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/messages", body)
	result, err := svc.Forward(context.Background(), c, geminiCompatThinkingTestAccount(), body)
	require.NoError(t, err)
	require.Len(t, upstream.requestBodies, 2)
	for _, sent := range upstream.requestBodies {
		require.Equal(t, "xhigh", gjson.GetBytes(sent, "generationConfig.thinkingConfig.thinkingLevel").String())
	}
	require.Equal(t, "xhigh", optionalStringValue(result.ReasoningEffort))
}

func TestGeminiThinkingEffortLaffey_UnsupportedLevelIsNotSilentlyDowngraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":400,"message":"Invalid thinkingLevel xhigh","status":"INVALID_ARGUMENT"}}`)),
	}}}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	body := []byte(`{"model":"gemini-4-carbon","reasoning_effort":"xhigh","messages":[{"role":"user","content":"hello"}]}`)
	c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", body)
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiCompatThinkingTestAccount(), body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.requestBodies, 1, "unsupported tiers must not retry a lower level")
	require.Equal(t, "xhigh", gjson.GetBytes(upstream.requestBodies[0], "generationConfig.thinkingConfig.thinkingLevel").String())
}
