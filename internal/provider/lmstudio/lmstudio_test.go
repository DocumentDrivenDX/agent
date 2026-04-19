package lmstudio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func lmStudioServer(loaded, max int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v0/models/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                    strings.TrimPrefix(r.URL.Path, "/api/v0/models/"),
			"loaded_context_length": loaded,
			"max_context_length":    max,
		})
	}))
}

func TestLookupModelLimits_PrefersLoadedContextLength(t *testing.T) {
	srv := lmStudioServer(100_000, 131_072)
	defer srv.Close()

	got := LookupModelLimits(context.Background(), srv.URL+"/v1", "qwen3.5-27b")
	assert.Equal(t, 100_000, got.ContextLength)
	assert.Equal(t, 0, got.MaxCompletionTokens)
}

func TestLookupModelLimits_FallsBackToMaxContextLength(t *testing.T) {
	srv := lmStudioServer(0, 131_072)
	defer srv.Close()

	got := LookupModelLimits(context.Background(), srv.URL+"/v1", "qwen3.5-27b")
	assert.Equal(t, 131_072, got.ContextLength)
}

func TestProtocolCapabilities(t *testing.T) {
	p := New(Config{BaseURL: "http://localhost:1234/v1"})
	assert.True(t, p.SupportsTools())
	assert.True(t, p.SupportsStream())
	assert.True(t, p.SupportsStructuredOutput())
	assert.True(t, p.SupportsThinking())
}
