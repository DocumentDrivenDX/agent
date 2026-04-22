package agent

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/DocumentDrivenDX/agent/internal/harnesses"
)

func TestResolveRouteExplicitHarnessModelIncompatible(t *testing.T) {
	svc := testRoutingErrorService()

	_, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Harness: "gemini",
		Model:   "minimax/minimax-m2.7",
	})
	if err == nil {
		t.Fatal("expected explicit harness/model incompatibility")
	}
	if !errors.Is(err, ErrHarnessModelIncompatible{}) {
		t.Fatalf("errors.Is should match ErrHarnessModelIncompatible: %T %v", err, err)
	}
	var typed *ErrHarnessModelIncompatible
	if !errors.As(err, &typed) {
		t.Fatalf("errors.As should extract ErrHarnessModelIncompatible: %T %v", err, err)
	}
	if typed.Harness != "gemini" {
		t.Fatalf("Harness=%q, want gemini", typed.Harness)
	}
	if typed.Model != "minimax/minimax-m2.7" {
		t.Fatalf("Model=%q, want minimax/minimax-m2.7", typed.Model)
	}
	want := []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"}
	if !slices.Equal(typed.SupportedModels, want) {
		t.Fatalf("SupportedModels=%v, want %v", typed.SupportedModels, want)
	}
}

func TestResolveRouteExplicitProfilePinConflict(t *testing.T) {
	svc := testRoutingErrorService()

	_, err := svc.ResolveRoute(context.Background(), RouteRequest{
		Profile: "local",
		Harness: "claude",
	})
	if err == nil {
		t.Fatal("expected local profile to conflict with claude harness")
	}
	if !errors.Is(err, ErrProfilePinConflict{}) {
		t.Fatalf("errors.Is should match ErrProfilePinConflict: %T %v", err, err)
	}
	var typed *ErrProfilePinConflict
	if !errors.As(err, &typed) {
		t.Fatalf("errors.As should extract ErrProfilePinConflict: %T %v", err, err)
	}
	if typed.Profile != "local" || typed.ConflictingPin != "Harness=claude" || typed.ProfileConstraint != "local-only" {
		t.Fatalf("profile conflict=%#v, want local/Harness=claude/local-only", typed)
	}

	_, err = svc.ResolveRoute(context.Background(), RouteRequest{
		Profile: "smart",
		Harness: "agent",
	})
	if err == nil {
		t.Fatal("expected smart profile to conflict with local agent harness")
	}
	var inverse *ErrProfilePinConflict
	if !errors.As(err, &inverse) {
		t.Fatalf("errors.As inverse: %T %v", err, err)
	}
	if inverse.Profile != "smart" || inverse.ConflictingPin != "Harness=agent" || inverse.ProfileConstraint != "subscription-only" {
		t.Fatalf("inverse profile conflict=%#v, want smart/Harness=agent/subscription-only", inverse)
	}
}

func testRoutingErrorService() *service {
	registry := harnesses.NewRegistry()
	registry.LookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	return &service{
		opts:     ServiceOptions{},
		registry: registry,
		hub:      newSessionHub(),
	}
}
