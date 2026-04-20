package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agent "github.com/DocumentDrivenDX/agent"
)

func TestExecute_DispatchesAdditionalSubprocessHarnesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake harness scripts rely on POSIX shell")
	}
	binDir := t.TempDir()
	writeFakeHarness(t, binDir, "gemini", `#!/bin/sh
cat <<'EOF'
gemini service response
{"stats":{"models":{"gemini":{"tokens":{"input":2,"total":5}}}}}
EOF
`)
	writeFakeHarness(t, binDir, "opencode", `#!/bin/sh
cat <<'EOF'
opencode service response
EOF
`)
	writeFakeHarness(t, binDir, "pi", `#!/bin/sh
cat <<'EOF'
{"type":"text_delta","partial":{"usage":{"input":3,"output":2,"cost":{"total":0.001}}}}
{"type":"text_end","message":{"usage":{"input":3,"output":2,"cost":{"total":0.001}}},"response":"pi service response"}
EOF
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	svc, err := agent.New(agent.ServiceOptions{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, tc := range []struct {
		harness string
		text    string
	}{
		{harness: "gemini", text: "gemini service response"},
		{harness: "opencode", text: "opencode service response"},
		{harness: "pi", text: "pi service response"},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ch, err := svc.Execute(ctx, agent.ServiceExecuteRequest{
				Prompt:      "hello",
				Harness:     tc.harness,
				Model:       "fake-model",
				WorkDir:     t.TempDir(),
				Permissions: "safe",
				Reasoning:   agent.ReasoningLow,
				Metadata:    map[string]string{"harness": tc.harness},
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			result, err := agent.DrainExecute(ctx, ch)
			if err != nil {
				t.Fatalf("DrainExecute: %v", err)
			}
			if result.FinalStatus != "success" {
				t.Fatalf("FinalStatus: got %q (err=%q)", result.FinalStatus, result.TerminalError)
			}
			if !strings.Contains(result.FinalText, tc.text) {
				t.Fatalf("FinalText: got %q, want to contain %q", result.FinalText, tc.text)
			}
			if result.RoutingActual == nil || result.RoutingActual.Harness != tc.harness {
				t.Fatalf("RoutingActual: got %#v, want harness %q", result.RoutingActual, tc.harness)
			}
		})
	}
}

func TestExecute_SubprocessHarnessMissingBinaryFinalFailure(t *testing.T) {
	svc, err := agent.New(agent.ServiceOptions{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Setenv("PATH", t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := svc.Execute(ctx, agent.ServiceExecuteRequest{
		Prompt:  "hello",
		Harness: "gemini",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, err := agent.DrainExecute(ctx, ch)
	if err != nil {
		t.Fatalf("DrainExecute: %v", err)
	}
	if result.FinalStatus != "failed" {
		t.Fatalf("FinalStatus: got %q", result.FinalStatus)
	}
	if !strings.Contains(result.TerminalError, "gemini binary not found") {
		t.Fatalf("TerminalError: got %q", result.TerminalError)
	}
}

func writeFakeHarness(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake harness %s: %v", name, err)
	}
}
