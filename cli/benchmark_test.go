package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func getRepoRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	return filepath.Dir(wd)
}

// TestBenchmarkRunnerPlanExpansion verifies --plan output for profile×task×rep expansion
// including tasks_from resolution.
func TestBenchmarkRunnerPlanExpansion(t *testing.T) {
	repoRoot := getRepoRoot(t)
	benchmarkScript := filepath.Join(repoRoot, "scripts", "benchmark", "benchmark")
	benchDir := filepath.Join(repoRoot, "scripts", "benchmark")

	if _, err := os.Stat(benchmarkScript); err != nil {
		t.Fatalf("benchmark script not found: %v", err)
	}

	tests := []struct {
		name        string
		profile     string
		benchSet    string
		expectRows  int
		shouldError bool
	}{
		{
			name:       "simple canary plan",
			profile:    "claude-sonnet-4-6",
			benchSet:   "tb-2-1-canary",
			expectRows: 9, // 1 profile × 3 tasks × 3 reps
		},
		{
			name:        "missing profile",
			profile:     "nonexistent-profile",
			benchSet:    "tb-2-1-canary",
			shouldError: true,
		},
		{
			name:        "missing bench-set",
			profile:     "claude-sonnet-4-6",
			benchSet:    "nonexistent-bench-set",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(benchmarkScript,
				"--profile", tt.profile,
				"--bench-set", tt.benchSet,
				"--plan")
			cmd.Dir = benchDir

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but command succeeded")
				}
				return
			}

			if err != nil {
				t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
			}

			// Parse output: each line is tab-separated key=value pairs
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
				t.Fatalf("no output from --plan")
			}

			if len(lines) != tt.expectRows {
				t.Errorf("expected %d rows, got %d\noutput: %s",
					tt.expectRows, len(lines), stdout.String())
			}

			// Validate each line has expected fields
			for i, line := range lines {
				fields := strings.Split(line, "\t")
				expectedFields := map[string]bool{
					"profile":       false,
					"bench_set":     false,
					"framework":     false,
					"dataset":       false,
					"task":          false,
					"rep":           false,
					"task_executor": false,
				}

				for _, field := range fields {
					parts := strings.SplitN(field, "=", 2)
					if len(parts) == 2 {
						expectedFields[parts[0]] = true
					}
				}

				for key, found := range expectedFields {
					if !found {
						t.Errorf("row %d missing field '%s': %s", i, key, line)
					}
				}
			}
		})
	}
}

// TestBenchmarkRunnerArgParsing verifies correct handling of unknown subcommands
// and missing required flags.
func TestBenchmarkRunnerArgParsing(t *testing.T) {
	repoRoot := getRepoRoot(t)
	benchmarkScript := filepath.Join(repoRoot, "scripts", "benchmark", "benchmark")
	benchDir := filepath.Join(repoRoot, "scripts", "benchmark")

	if _, err := os.Stat(benchmarkScript); err != nil {
		t.Fatalf("benchmark script not found: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		shouldError bool
		checkOutput func(t *testing.T, stderr string)
	}{
		{
			name:        "unknown subcommand",
			args:        []string{"unknown-subcommand"},
			shouldError: true,
			checkOutput: func(t *testing.T, stderr string) {
				if !strings.Contains(stderr, "unknown subcommand") {
					t.Errorf("expected 'unknown subcommand' in stderr, got: %s", stderr)
				}
			},
		},
		{
			name:        "missing --profile flag",
			args:        []string{"--bench-set", "tb-2-1-canary", "--plan"},
			shouldError: true,
			checkOutput: func(t *testing.T, stderr string) {
				if !strings.Contains(stderr, "--profile") {
					t.Errorf("expected '--profile' mention in stderr, got: %s", stderr)
				}
			},
		},
		{
			name:        "missing --bench-set flag",
			args:        []string{"--profile", "claude-sonnet-4-6", "--plan"},
			shouldError: true,
			checkOutput: func(t *testing.T, stderr string) {
				if !strings.Contains(stderr, "--bench-set") {
					t.Errorf("expected '--bench-set' mention in stderr, got: %s", stderr)
				}
			},
		},
		{
			name:        "unknown flag",
			args:        []string{"--unknown-flag", "value"},
			shouldError: true,
			checkOutput: func(t *testing.T, stderr string) {
				if !strings.Contains(stderr, "unknown") {
					t.Errorf("expected 'unknown' in stderr, got: %s", stderr)
				}
			},
		},
		{
			name:        "valid subcommand: profiles",
			args:        []string{"profiles"},
			shouldError: false,
		},
		{
			name:        "valid subcommand: bench-sets",
			args:        []string{"bench-sets"},
			shouldError: false,
		},
		{
			name:        "valid subcommand: task-executors",
			args:        []string{"task-executors"},
			shouldError: false,
		},
		{
			name:        "valid subcommand: harness-adapters",
			args:        []string{"harness-adapters"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(benchmarkScript, tt.args...)
			cmd.Dir = benchDir

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but command succeeded")
				}
			} else {
				if err != nil {
					t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
				}
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, stderr.String())
			}
		})
	}
}

// TestBenchmarkRunnerMatrixJSON verifies --plan output is correctly formatted
// as key=value pairs that can be parsed into structured data.
func TestBenchmarkRunnerMatrixJSON(t *testing.T) {
	repoRoot := getRepoRoot(t)
	benchmarkScript := filepath.Join(repoRoot, "scripts", "benchmark", "benchmark")
	benchDir := filepath.Join(repoRoot, "scripts", "benchmark")

	if _, err := os.Stat(benchmarkScript); err != nil {
		t.Fatalf("benchmark script not found: %v", err)
	}

	cmd := exec.Command(benchmarkScript,
		"--profile", "claude-sonnet-4-6",
		"--bench-set", "tb-2-1-canary",
		"--plan")
	cmd.Dir = benchDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("no output from --plan")
	}

	// Verify each line can be parsed as key=value pairs
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		for _, field := range fields {
			if !strings.Contains(field, "=") {
				t.Errorf("row %d field not in key=value format: %s", i, field)
				continue
			}
		}
	}

	// Verify specific cells in the plan
	// With 1 profile, 3 tasks, 3 reps: should have specific pattern
	if len(lines) != 9 {
		t.Errorf("expected 9 cells (1 profile × 3 tasks × 3 reps), got %d", len(lines))
	}

	// Verify rep counting (should go from 1/3 to 3/3 for each task)
	repCounts := make(map[string]int)
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		for _, field := range fields {
			if strings.HasPrefix(field, "rep=") {
				repCounts[field]++
			}
		}
	}

	expectedReps := []string{"rep=1/3", "rep=2/3", "rep=3/3"}
	for _, rep := range expectedReps {
		if repCounts[rep] != 3 {
			t.Errorf("expected 3 cells with %s, got %d", rep, repCounts[rep])
		}
	}
}
