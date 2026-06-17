package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerGroupPayloadEmptyReturnsLiteralNil(t *testing.T) {
	emptyGroupsPayload := buildSchedulerGroupPayload(nil)
	require.Nil(t, emptyGroupsPayload)
}
