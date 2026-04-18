//go:build testseam

package agent_test

import (
	"testing"

	"github.com/DocumentDrivenDX/agent"
)

// TestSeamFakeProvider demonstrates that FakeProvider compiles and can be
// constructed with all three patterns under the testseam build tag.
func TestSeamFakeProvider(t *testing.T) {
	callCount := 0

	fp := &agent.FakeProvider{
		Static: []agent.FakeResponse{
			{Text: "hello", Status: "success"},
		},
		Dynamic: func(req agent.FakeRequest) (agent.FakeResponse, error) {
			return agent.FakeResponse{Text: "dynamic", Status: "success"}, nil
		},
		InjectError: func(callIndex int) error {
			callCount++
			return nil
		},
	}

	opts := agent.Options{}
	opts.FakeProvider = fp

	if opts.FakeProvider == nil {
		t.Fatal("FakeProvider should not be nil")
	}
	if len(opts.FakeProvider.Static) != 1 {
		t.Fatalf("expected 1 static response, got %d", len(opts.FakeProvider.Static))
	}
	if opts.FakeProvider.Dynamic == nil {
		t.Fatal("Dynamic should not be nil")
	}
	if opts.FakeProvider.InjectError == nil {
		t.Fatal("InjectError should not be nil")
	}
	_ = callCount
}

// TestSeamPromptAssertionHook demonstrates that PromptAssertionHook compiles
// and captures the system+user prompt and context files.
func TestSeamPromptAssertionHook(t *testing.T) {
	captured := struct {
		systemPrompt string
		userPrompt   string
		contextFiles []string
	}{}

	hook := agent.PromptAssertionHook(func(systemPrompt, userPrompt string, contextFiles []string) {
		captured.systemPrompt = systemPrompt
		captured.userPrompt = userPrompt
		captured.contextFiles = contextFiles
	})

	opts := agent.Options{}
	opts.PromptAssertionHook = hook

	if opts.PromptAssertionHook == nil {
		t.Fatal("PromptAssertionHook should not be nil")
	}

	// Invoke the hook directly to confirm the capture works.
	opts.PromptAssertionHook("sys", "usr", []string{"file.go"})

	if captured.systemPrompt != "sys" {
		t.Fatalf("expected systemPrompt=sys, got %q", captured.systemPrompt)
	}
	if captured.userPrompt != "usr" {
		t.Fatalf("expected userPrompt=usr, got %q", captured.userPrompt)
	}
	if len(captured.contextFiles) != 1 || captured.contextFiles[0] != "file.go" {
		t.Fatalf("unexpected contextFiles: %v", captured.contextFiles)
	}
}

// TestSeamCompactionAssertionHook demonstrates that CompactionAssertionHook
// compiles and captures compaction metrics.
func TestSeamCompactionAssertionHook(t *testing.T) {
	var gotBefore, gotAfter, gotFreed int

	hook := agent.CompactionAssertionHook(func(messagesBefore, messagesAfter int, tokensFreed int) {
		gotBefore = messagesBefore
		gotAfter = messagesAfter
		gotFreed = tokensFreed
	})

	opts := agent.Options{}
	opts.CompactionAssertionHook = hook

	if opts.CompactionAssertionHook == nil {
		t.Fatal("CompactionAssertionHook should not be nil")
	}

	opts.CompactionAssertionHook(30, 12, 4521)

	if gotBefore != 30 || gotAfter != 12 || gotFreed != 4521 {
		t.Fatalf("unexpected values: before=%d after=%d freed=%d", gotBefore, gotAfter, gotFreed)
	}
}

// TestSeamToolWiringHook demonstrates that ToolWiringHook compiles and
// captures the harness name + resolved tool list.
func TestSeamToolWiringHook(t *testing.T) {
	var gotHarness string
	var gotTools []string

	hook := agent.ToolWiringHook(func(harness string, toolNames []string) {
		gotHarness = harness
		gotTools = toolNames
	})

	opts := agent.Options{}
	opts.ToolWiringHook = hook

	if opts.ToolWiringHook == nil {
		t.Fatal("ToolWiringHook should not be nil")
	}

	opts.ToolWiringHook("agent", []string{"bash", "read_file", "write_file"})

	if gotHarness != "agent" {
		t.Fatalf("expected harness=agent, got %q", gotHarness)
	}
	if len(gotTools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(gotTools), gotTools)
	}
}
