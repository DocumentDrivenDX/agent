package agentcli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_UsesInjectedOutputAndDoesNotExit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(Options{
		Args:      []string{"--version"},
		Stdout:    &stdout,
		Stderr:    &stderr,
		Version:   "v-test",
		BuildTime: "2026-04-30T00:00:00Z",
		GitCommit: "abc123",
	})
	if code != 0 {
		t.Fatalf("Run exit = %d, want 0", code)
	}
	if got := stdout.String(); !strings.Contains(got, "fiz v-test") || !strings.Contains(got, "abc123") {
		t.Fatalf("stdout = %q, want injected version output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
