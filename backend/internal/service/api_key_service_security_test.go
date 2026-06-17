package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeySecurityRepoStub struct {
	APIKeyRepository
	created *APIKey
	updated *APIKey
	stored  *APIKey
}

func (r *apiKeySecurityRepoStub) Create(_ context.Context, key *APIKey) error {
	clone := *key
	r.created = &clone
	return nil
}

func (r *apiKeySecurityRepoStub) GetByID(_ context.Context, _ int64) (*APIKey, error) {
	if r.stored == nil {
		return nil, ErrAPIKeyNotFound
	}
	clone := *r.stored
	return &clone, nil
}

func (r *apiKeySecurityRepoStub) Update(_ context.Context, key *APIKey) error {
	clone := *key
	r.updated = &clone
	return nil
}

func (r *apiKeySecurityRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}

func (r *apiKeySecurityRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *apiKeySecurityRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

type apiKeySecurityUserRepoStub struct {
	UserRepository
	user *User
}

func (r *apiKeySecurityUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if r.user == nil {
		return nil, ErrUserNotFound
	}
	clone := *r.user
	return &clone, nil
}

func TestAPIKeyService_CreateEscapesName(t *testing.T) {
	repo := &apiKeySecurityRepoStub{}
	userRepo := &apiKeySecurityUserRepoStub{user: &User{ID: 7, Status: StatusActive, Role: RoleUser}}
	svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})

	got, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{
		Name: `<img src=x onerror=alert(1)>`,
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.created)
	require.Equal(t, `&lt;img src=x onerror=alert(1)&gt;`, repo.created.Name)
	require.Equal(t, repo.created.Name, got.Name)
}

func TestAPIKeyService_UpdateEscapesName(t *testing.T) {
	rawName := `<script>alert(1)</script>`
	repo := &apiKeySecurityRepoStub{
		stored: &APIKey{ID: 10, UserID: 7, Key: "sk-existing", Name: "old", Status: StatusActive},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	got, err := svc.Update(context.Background(), 10, 7, UpdateAPIKeyRequest{
		Name: &rawName,
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.updated)
	require.Equal(t, `&lt;script&gt;alert(1)&lt;/script&gt;`, repo.updated.Name)
	require.Equal(t, repo.updated.Name, got.Name)
}
