package omlx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func omlxServer(modelID string, maxContext, maxTokens int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{
					"id":                 modelID,
					"max_context_window": maxContext,
					"max_tokens":         maxTokens,
				},
			},
		})
	}))
}

func TestLookupModelLimits(t *testing.T) {
	srv := omlxServer("Qwen3.5-27B-4bit", 262_144, 32_768)
	defer srv.Close()

	got := LookupModelLimits(context.Background(), srv.URL+"/v1", "Qwen3.5-27B-4bit")
	assert.Equal(t, 262_144, got.ContextLength)
	assert.Equal(t, 32_768, got.MaxCompletionTokens)
}

func TestLookupModelLimits_ModelMatchIsCaseInsensitive(t *testing.T) {
	srv := omlxServer("Qwen3.5-27B-4bit", 262_144, 32_768)
	defer srv.Close()

	got := LookupModelLimits(context.Background(), srv.URL+"/v1", "qwen3.5-27b-4bit")
	assert.Equal(t, 262_144, got.ContextLength)
}

func TestLookupModelLimits_UnknownModelReturnsZero(t *testing.T) {
	srv := omlxServer("foo-model", 262_144, 32_768)
	defer srv.Close()

	got := LookupModelLimits(context.Background(), srv.URL+"/v1", "bar-model")
	assert.Zero(t, got.ContextLength)
	assert.Zero(t, got.MaxCompletionTokens)
}

func TestProtocolCapabilities(t *testing.T) {
	p := New(Config{BaseURL: "http://localhost:1235/v1"})
	assert.True(t, p.SupportsTools())
	assert.True(t, p.SupportsStream())
	assert.True(t, p.SupportsStructuredOutput())
	assert.False(t, p.SupportsThinking())
}
