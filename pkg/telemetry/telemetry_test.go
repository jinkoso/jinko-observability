package telemetry_test

import (
	"context"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_DisabledIsNoop(t *testing.T) {
	shutdown, err := telemetry.Init(context.Background(), telemetry.Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// No-op shutdown must not error and must be idempotent.
	assert.NoError(t, shutdown(context.Background()))
	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_EnabledRequiresServiceName(t *testing.T) {
	_, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:      true,
		OTLPEndpoint: "localhost:4317",
		Insecure:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ServiceName")
}

func TestInit_EnabledRequiresOTLPEndpoint(t *testing.T) {
	_, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:     true,
		ServiceName: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTLPEndpoint")
}
