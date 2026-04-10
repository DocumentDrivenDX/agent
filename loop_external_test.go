package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	agent "github.com/DocumentDrivenDX/agent"
	openaiprovider "github.com/DocumentDrivenDX/agent/provider/openai"
	"github.com/DocumentDrivenDX/agent/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRun_FailedOpenAICompatibleChatSpansIncludeServerIdentity(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tel := telemetry.New(telemetry.Config{TracerProvider: tp})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(parsed.Port())
	require.NoError(t, err)

	provider := openaiprovider.New(openaiprovider.Config{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test",
		Model:   "gpt-4o",
	})

	result, err := agent.Run(context.Background(), agent.Request{
		Prompt:    "trigger failure",
		Provider:  provider,
		Telemetry: tel,
		NoStream:  true,
	})
	require.Error(t, err)
	assert.Equal(t, agent.StatusError, result.Status)

	ended := recorder.Ended()
	chatSpans := spansWithOperation(ended, "chat")
	require.Len(t, chatSpans, 3)

	for _, span := range chatSpans {
		assert.Equal(t, "openai-compat", attrString(span.Attributes(), telemetry.KeyProviderName))
		assert.Equal(t, "local", attrString(span.Attributes(), telemetry.KeyProviderSystem))
		assert.Equal(t, parsed.Hostname(), attrString(span.Attributes(), telemetry.KeyServerAddress))
		assert.Equal(t, int64(port), attrInt(span.Attributes(), telemetry.KeyServerPort))
	}
}

func spansWithOperation(spans []sdktrace.ReadOnlySpan, operation string) []sdktrace.ReadOnlySpan {
	var filtered []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if value, ok := attrStringOk(span.Attributes(), telemetry.KeyOperationName); ok && value == operation {
			filtered = append(filtered, span)
		}
	}
	return filtered
}

func attrString(attrs []attribute.KeyValue, key string) string {
	value, _ := attrStringOk(attrs, key)
	return value
}

func attrStringOk(attrs []attribute.KeyValue, key string) (string, bool) {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value.AsString(), true
		}
	}
	return "", false
}

func attrInt(attrs []attribute.KeyValue, key string) int64 {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value.AsInt64()
		}
	}
	return 0
}
