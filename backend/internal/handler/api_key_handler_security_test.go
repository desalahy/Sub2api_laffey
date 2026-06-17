package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiKeyHandlerSecurityRepoStub struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *apiKeyHandlerSecurityRepoStub) GetByID(context.Context, int64) (*service.APIKey, error) {
	if r.key == nil {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *r.key
	return &clone, nil
}

func TestAPIKeyHandlerGetByIDReturnsNotFoundForOtherUsersKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &apiKeyHandlerSecurityRepoStub{
		key: &service.APIKey{ID: 99, UserID: 1001, Key: "sk-other", Status: service.StatusActive},
	}
	svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewAPIKeyHandler(svc)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "99"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/99", nil)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.GetByID(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "API key not found")
}
