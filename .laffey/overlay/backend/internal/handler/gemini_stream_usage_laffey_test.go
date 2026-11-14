//go:build unit

package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiUsageUpstreamLaffey struct {
	service.HTTPUpstream
	body string
}

func (u *geminiUsageUpstreamLaffey) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}

func TestGeminiMessagesLaffey_ProtocolErrorSettlesObservedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	groupID := int64(9300)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformGemini, Status: service.StatusActive, RateMultiplier: 1}
	account := &service.Account{ID: 9301, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "test-key"}, AccountGroups: []service.AccountGroup{{AccountID: 9301, GroupID: groupID}}}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
	billingRepo := &compatStreamUsageBillingRepo{}
	billingCache := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	upstream := &geminiUsageUpstreamLaffey{body: `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"Read","args":[]}}]}}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}` + "\n\n"}
	gw := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usageRepo, billingRepo, nil, nil, nil, nil, cfg, snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	gemini := service.NewGeminiMessagesCompatService(nil, &fakeGroupRepo{group: group}, nil, snapshot, nil, nil, upstream, nil, cfg)
	h := &GatewayHandler{gatewayService: gw, geminiCompatService: gemini, billingCacheService: billingCache, cfg: cfg, maxAccountSwitchesGemini: 1, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"gemini-2.5-flash","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"read"}]}`))
	apiKey := &service.APIKey{ID: 9302, UserID: 9303, GroupID: &groupID, Group: group, User: &service.User{ID: 9303, Balance: 100}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9303})
	h.Messages(c)
	require.Contains(t, w.Body.String(), `"type":"upstream_error"`)
	require.NotContains(t, w.Body.String(), `"type":"message_stop"`)
	billingRepo.mu.Lock()
	defer billingRepo.mu.Unlock()
	require.Len(t, billingRepo.commands, 1, "observed usage must be settled exactly once")
	require.Equal(t, 7, billingRepo.commands[0].InputTokens)
	require.Equal(t, 3, billingRepo.commands[0].OutputTokens)
}
