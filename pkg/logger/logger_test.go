package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	otrace "go.opentelemetry.io/otel/trace"

	"github.com/jinkoso/jinko-observability/pkg/logger"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	var (
		buf bytes.Buffer
		wg  sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()

	fn()
	_ = w.Close()
	os.Stdout = orig
	wg.Wait()
	return buf.String()
}

func decodeJSONLines(t *testing.T, raw string) []map[string]any {
	t.Helper()

	var out []map[string]any
	for _, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		out = append(out, entry)
	}
	return out
}

func TestNew_DefaultConfig(t *testing.T) {
	log, err := logger.New(logger.DefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, log)
}

func TestNew_InvalidLevel(t *testing.T) {
	_, err := logger.New(logger.Config{Level: "bogus", Format: "json"})
	require.Error(t, err)
}

func TestNew_InvalidFormat(t *testing.T) {
	_, err := logger.New(logger.Config{Level: "info", Format: "yaml"})
	require.Error(t, err)
}

func TestNew_InvalidOutput(t *testing.T) {
	_, err := logger.New(logger.Config{Level: "info", Format: "json", Output: "syslog"})
	require.Error(t, err)
}

func TestInfo_EmitsStructuredJSON(t *testing.T) {
	out := captureStdout(t, func() {
		log, err := logger.New(logger.DefaultConfig())
		require.NoError(t, err)
		log.Info(context.Background(), "hello",
			logger.String("service", "test"),
			logger.Int("port", 8080),
			logger.Bool("ready", true),
		)
	})

	entries := decodeJSONLines(t, out)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "hello", e["message"])
	assert.Equal(t, "info", e["level"])
	assert.Equal(t, "test", e["service"])
	assert.EqualValues(t, 8080, e["port"])
	assert.Equal(t, true, e["ready"])
}

func TestWith_PrependsFieldsToEveryEntry(t *testing.T) {
	out := captureStdout(t, func() {
		log, err := logger.New(logger.DefaultConfig())
		require.NoError(t, err)
		scoped := log.With(logger.String("tenant", "acme"))
		scoped.Info(context.Background(), "one")
		scoped.Warn(context.Background(), "two")
	})

	entries := decodeJSONLines(t, out)
	require.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, "acme", e["tenant"])
	}
}

func TestWithError_AddsErrorField(t *testing.T) {
	out := captureStdout(t, func() {
		log, err := logger.New(logger.DefaultConfig())
		require.NoError(t, err)
		log.WithError(errors.New("boom")).Error(context.Background(), "fail")
	})

	entries := decodeJSONLines(t, out)
	require.Len(t, entries, 1)
	assert.Equal(t, "boom", entries[0]["error"])
}

func TestLog_IncludesOTelAndDatadogTraceIDsWhenSpanPresent(t *testing.T) {
	tp := trace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	sc := otrace.SpanContextFromContext(ctx)
	require.True(t, sc.IsValid(), "span context must be valid for the test")

	out := captureStdout(t, func() {
		log, err := logger.New(logger.DefaultConfig())
		require.NoError(t, err)
		log.Info(ctx, "traced")
	})

	entries := decodeJSONLines(t, out)
	require.Len(t, entries, 1)
	e := entries[0]

	assert.Equal(t, sc.TraceID().String(), e["trace_id"])
	assert.Equal(t, sc.SpanID().String(), e["span_id"])
	assert.NotEmpty(t, e["dd.trace_id"], "dd.trace_id must be set for Datadog correlation")
	assert.NotEmpty(t, e["dd.span_id"], "dd.span_id must be set for Datadog correlation")
}

func TestLog_OmitsTraceFieldsWhenNoSpan(t *testing.T) {
	out := captureStdout(t, func() {
		log, err := logger.New(logger.DefaultConfig())
		require.NoError(t, err)
		log.Info(context.Background(), "no span")
	})

	entries := decodeJSONLines(t, out)
	require.Len(t, entries, 1)
	_, hasTrace := entries[0]["trace_id"]
	_, hasDD := entries[0]["dd.trace_id"]
	assert.False(t, hasTrace)
	assert.False(t, hasDD)
}

func TestNoop_DiscardsEntries(t *testing.T) {
	out := captureStdout(t, func() {
		n := logger.Noop()
		n.Info(context.Background(), "should not appear",
			logger.String("key", "value"),
		)
		n.With(logger.String("scope", "x")).Error(context.Background(), "also discarded")
		n.WithError(errors.New("ignored")).Warn(context.Background(), "still discarded")
	})

	assert.Empty(t, out)
}

func TestExtractTraceContext_Empty(t *testing.T) {
	tc := logger.ExtractTraceContext(context.Background())
	assert.Empty(t, tc.TraceID)
	assert.Empty(t, tc.SpanID)
}

func TestTraceFieldsFromContext_EmptyWhenNoSpan(t *testing.T) {
	assert.Nil(t, logger.TraceFieldsFromContext(context.Background()))
}

func TestMustNew_PanicsOnBadConfig(t *testing.T) {
	assert.Panics(t, func() {
		_ = logger.MustNew(logger.Config{Level: "nope"})
	})
}
