package agent

import (
	"context"
	"testing"
	"time"

	"github.com/DocumentDrivenDX/agent/internal/harnesses"
)

func TestRecordRouteAttempt_DemotesFailedProvider(t *testing.T) {
	svc := routeAttemptTestService(30 * time.Second)

	before, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Model:    "qwen",
		Provider: "bragi",
	})
	if err != nil {
		t.Fatalf("ResolveRoute before failure: %v", err)
	}
	if before.Provider != "bragi" {
		t.Fatalf("before failure Provider: got %q, want bragi", before.Provider)
	}

	if err := svc.RecordRouteAttempt(context.Background(), RouteAttempt{
		Harness:  "agent",
		Provider: "bragi",
		Model:    "qwen",
		Status:   "failed",
		Reason:   "timeout",
		Error:    "context deadline exceeded",
	}); err != nil {
		t.Fatalf("RecordRouteAttempt: %v", err)
	}

	after, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Model:    "qwen",
		Provider: "bragi",
	})
	if err != nil {
		t.Fatalf("ResolveRoute after failure: %v", err)
	}
	if after.Provider == "bragi" {
		t.Fatalf("after failure Provider: got bragi, want a non-cooldown provider")
	}
}

func TestRecordRouteAttempt_SuccessClearsFailure(t *testing.T) {
	svc := routeAttemptTestService(30 * time.Second)
	if err := svc.RecordRouteAttempt(context.Background(), RouteAttempt{
		Harness:  "agent",
		Provider: "bragi",
		Model:    "qwen",
		Status:   "failed",
		Error:    "502 bad gateway",
	}); err != nil {
		t.Fatalf("RecordRouteAttempt failed: %v", err)
	}
	if err := svc.RecordRouteAttempt(context.Background(), RouteAttempt{
		Harness:  "agent",
		Provider: "bragi",
		Model:    "qwen",
		Status:   "success",
	}); err != nil {
		t.Fatalf("RecordRouteAttempt success: %v", err)
	}

	dec, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Model:    "qwen",
		Provider: "bragi",
	})
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}
	if dec.Provider != "bragi" {
		t.Fatalf("Provider after success clear: got %q, want bragi", dec.Provider)
	}
}

func TestRecordRouteAttempt_TTLExpiryRemovesDemotion(t *testing.T) {
	svc := routeAttemptTestService(10 * time.Millisecond)
	if err := svc.RecordRouteAttempt(context.Background(), RouteAttempt{
		Harness:   "agent",
		Provider:  "bragi",
		Model:     "qwen",
		Status:    "failed",
		Timestamp: time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatalf("RecordRouteAttempt: %v", err)
	}

	dec, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Model:    "qwen",
		Provider: "bragi",
	})
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}
	if dec.Provider != "bragi" {
		t.Fatalf("Provider after TTL expiry: got %q, want bragi", dec.Provider)
	}
}

func TestRouteStatus_RouteAttemptCooldownSurfaces(t *testing.T) {
	svc := routeAttemptTestService(30 * time.Second)
	recordedAt := time.Now().Add(-time.Second).UTC()
	if err := svc.RecordRouteAttempt(context.Background(), RouteAttempt{
		Harness:   "agent",
		Provider:  "bragi",
		Model:     "qwen",
		Status:    "failed",
		Reason:    "rate_limit",
		Error:     "429 too many requests",
		Timestamp: recordedAt,
	}); err != nil {
		t.Fatalf("RecordRouteAttempt: %v", err)
	}

	report, err := svc.RouteStatus(context.Background())
	if err != nil {
		t.Fatalf("RouteStatus: %v", err)
	}
	if len(report.Routes) != 1 {
		t.Fatalf("Routes: got %d, want 1", len(report.Routes))
	}
	byProvider := make(map[string]RouteCandidateStatus)
	for _, cand := range report.Routes[0].Candidates {
		byProvider[cand.Provider] = cand
	}
	bragi := byProvider["bragi"]
	if bragi.Healthy {
		t.Fatal("bragi should be unhealthy while route-attempt cooldown is active")
	}
	if bragi.Cooldown == nil {
		t.Fatal("bragi cooldown should be populated")
	}
	if bragi.Cooldown.Reason != "rate_limit" {
		t.Fatalf("Cooldown.Reason: got %q, want rate_limit", bragi.Cooldown.Reason)
	}
	if bragi.Cooldown.LastError != "429 too many requests" {
		t.Fatalf("Cooldown.LastError: got %q", bragi.Cooldown.LastError)
	}
	if !bragi.Cooldown.LastAttempt.Equal(recordedAt) {
		t.Fatalf("Cooldown.LastAttempt: got %s, want %s", bragi.Cooldown.LastAttempt, recordedAt)
	}
	if !byProvider["openrouter"].Healthy {
		t.Fatal("openrouter should remain healthy")
	}
}

func routeAttemptTestService(cooldown time.Duration) *service {
	sc := &fakeServiceConfig{
		providers: map[string]ServiceProviderEntry{
			"bragi":      {Type: "lmstudio", BaseURL: "http://127.0.0.1:9999/v1", Model: "qwen"},
			"openrouter": {Type: "openrouter", BaseURL: "https://openrouter.invalid/v1", Model: "qwen"},
		},
		names:          []string{"bragi", "openrouter"},
		defaultName:    "bragi",
		healthCooldown: cooldown,
		routeConfigs: map[string]ServiceModelRouteConfig{
			"qwen": {
				Strategy: "ordered-failover",
				Candidates: []ServiceRouteCandidateEntry{
					{Provider: "bragi", Model: "qwen", Priority: 100},
					{Provider: "openrouter", Model: "qwen", Priority: 50},
				},
			},
		},
		routes: map[string][]string{"qwen": {"bragi", "openrouter"}},
	}
	return &service{opts: ServiceOptions{ServiceConfig: sc}, registry: harnesses.NewRegistry()}
}
