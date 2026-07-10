package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type laffeySettingRepo struct {
	values map[string]string
}

func (r *laffeySettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *laffeySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *laffeySettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *laffeySettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}

func (r *laffeySettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *laffeySettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *laffeySettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestNormalizeLaffeySiteName(t *testing.T) {
	t.Parallel()
	require.Equal(t, defaultSiteName, normalizeLaffeySiteName(""))
	require.Equal(t, defaultSiteName, normalizeLaffeySiteName("  Sub2API  "))
	require.Equal(t, "Custom Site", normalizeLaffeySiteName(" Custom Site "))
}

func TestResolveLaffeySiteNameMigratesLegacyDefault(t *testing.T) {
	t.Parallel()
	repo := &laffeySettingRepo{values: map[string]string{SettingKeySiteName: legacyDefaultSiteName}}

	actual := resolveLaffeySiteName(context.Background(), repo, legacyDefaultSiteName)

	require.Equal(t, defaultSiteName, actual)
	require.Equal(t, defaultSiteName, repo.values[SettingKeySiteName])
}
