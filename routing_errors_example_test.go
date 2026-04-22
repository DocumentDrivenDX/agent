package agent_test

import (
	"errors"
	"fmt"
	"strings"

	agent "github.com/DocumentDrivenDX/agent"
)

func ExampleErrHarnessModelIncompatible() {
	err := fmt.Errorf("ddx preflight: %w", &agent.ErrHarnessModelIncompatible{
		Harness:         "gemini",
		Model:           "minimax/minimax-m2.7",
		SupportedModels: []string{"gemini-2.5-pro", "gemini-2.5-flash"},
	})

	var routeErr *agent.ErrHarnessModelIncompatible
	if errors.As(err, &routeErr) {
		fmt.Printf("ddx failed bead: harness=%s model=%s supported=%s\n",
			routeErr.Harness,
			routeErr.Model,
			strings.Join(routeErr.SupportedModels, ","))
	}
	fmt.Println(errors.Is(err, agent.ErrHarnessModelIncompatible{}))

	// Output:
	// ddx failed bead: harness=gemini model=minimax/minimax-m2.7 supported=gemini-2.5-pro,gemini-2.5-flash
	// true
}

func ExampleErrProfilePinConflict() {
	err := fmt.Errorf("ddx preflight: %w", &agent.ErrProfilePinConflict{
		Profile:           "local",
		ConflictingPin:    "Harness=claude",
		ProfileConstraint: "local-only",
	})

	var routeErr *agent.ErrProfilePinConflict
	if errors.As(err, &routeErr) {
		fmt.Printf("ddx failed bead: profile=%s conflict=%s constraint=%s\n",
			routeErr.Profile,
			routeErr.ConflictingPin,
			routeErr.ProfileConstraint)
	}
	fmt.Println(errors.Is(err, agent.ErrProfilePinConflict{}))

	// Output:
	// ddx failed bead: profile=local conflict=Harness=claude constraint=local-only
	// true
}
