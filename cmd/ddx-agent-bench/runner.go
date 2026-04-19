package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agent "github.com/DocumentDrivenDX/agent"
	agentConfig "github.com/DocumentDrivenDX/agent/config"
	"github.com/DocumentDrivenDX/agent/internal/comparison"
	"github.com/DocumentDrivenDX/agent/internal/harnesses"
)

// buildRunFunc constructs a comparison.RunFunc that drives agent execution via
// service.Execute. The RunFunc signature is (harness, model, prompt) ->
// RunResult per CONTRACT-003.
func buildRunFunc(wd string, timeout time.Duration) (comparison.RunFunc, error) {
	cfg, err := agentConfig.Load(wd)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	svc, err := agent.New(agent.ServiceOptions{
		ServiceConfig: &configAdapter{cfg: cfg, workDir: wd},
	})
	if err != nil {
		return nil, fmt.Errorf("new service: %w", err)
	}

	return func(harness, model, prompt string) comparison.RunResult {
		result := comparison.RunResult{
			Harness: harness,
			Model:   model,
		}

		ctx := context.Background()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		start := time.Now()

		req := agent.ServiceExecuteRequest{
			Harness: harness,
			Model:   model,
			Prompt:  prompt,
			WorkDir: wd,
			// Use safe permissions for bench corpus tasks.
			Permissions: "safe",
		}

		ch, err := svc.Execute(ctx, req)
		if err != nil {
			result.Error = err.Error()
			result.ExitCode = 1
			result.DurationMS = int(time.Since(start).Milliseconds())
			return result
		}

		var outputBuf strings.Builder
		for ev := range ch {
			switch ev.Type {
			case harnesses.EventTypeTextDelta:
				var td harnesses.TextDeltaData
				if err := json.Unmarshal(ev.Data, &td); err == nil {
					outputBuf.WriteString(td.Text)
				}
			case harnesses.EventTypeToolCall:
				var tc harnesses.ToolCallData
				if err := json.Unmarshal(ev.Data, &tc); err == nil {
					var inputStr string
					if tc.Input != nil {
						inputStr = string(tc.Input)
					}
					result.ToolCalls = append(result.ToolCalls, comparison.ToolCallEntry{
						Tool:  tc.Name,
						Input: inputStr,
					})
				}
			case harnesses.EventTypeFinal:
				var fd harnesses.FinalData
				if err := json.Unmarshal(ev.Data, &fd); err == nil {
					result.ExitCode = fd.ExitCode
					result.Error = fd.Error
					if fd.Usage != nil {
						result.InputTokens = fd.Usage.InputTokens
						result.OutputTokens = fd.Usage.OutputTokens
						result.Tokens = fd.Usage.TotalTokens
					}
					result.CostUSD = fd.CostUSD
				}
			}
		}

		result.Output = outputBuf.String()
		result.DurationMS = int(time.Since(start).Milliseconds())
		return result
	}, nil
}

// cmdRun implements the 'run' subcommand.
func cmdRun(args []string) int {
	fs := flagSet("run")
	corpusDir := fs.String("corpus", "", "Corpus directory (default: bench/corpus relative to work-dir)")
	workDir := fs.String("work-dir", "", "Agent working directory (default: cwd)")
	jsonOut := fs.Bool("json", false, "Emit JSON results")
	harnessFilter := fs.String("harness", "", "Only run against this harness")
	maxCostUSD := fs.Float64("max-cost-usd", 0, "Cost cap in USD (stub; accepted but not enforced)")
	timeoutSec := fs.Int("timeout", 120, "Per-task timeout in seconds")
	resultsDir := fs.String("results-dir", "", "Directory to write result JSON (default: bench/results relative to work-dir)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *maxCostUSD > 0 {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench: note: --max-cost-usd is accepted but not yet enforced (see agent-741a3882)\n")
	}

	wd := resolveWorkDir(*workDir)

	// Resolve corpus directory.
	corpus := *corpusDir
	if corpus == "" {
		corpus = filepath.Join(wd, "bench", "corpus")
	}

	// Resolve results directory.
	outDir := *resultsDir
	if outDir == "" {
		outDir = filepath.Join(wd, "bench", "results")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: create results dir: %v\n", err)
		return 1
	}

	// Load corpus tasks.
	tasks, err := loadCorpus(corpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: load corpus: %v\n", err)
		return 1
	}
	if len(tasks) == 0 {
		fmt.Fprintln(os.Stderr, "ddx-agent-bench run: no tasks found in corpus")
		return 1
	}

	// Discover candidates.
	candidates, err := discoverCandidates(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: discover: %v\n", err)
		return 1
	}

	// Apply harness filter.
	if *harnessFilter != "" {
		var filtered []Candidate
		for _, c := range candidates {
			if c.Harness == *harnessFilter {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		fmt.Fprintln(os.Stderr, "ddx-agent-bench run: no candidates available")
		return 1
	}

	timeout := time.Duration(*timeoutSec) * time.Second
	runFn, err := buildRunFunc(wd, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: build runner: %v\n", err)
		return 1
	}

	// Build a BenchmarkSuite from corpus tasks + candidates.
	suite := buildSuite(tasks, candidates)
	result, err := comparison.RunBenchmark(runFn, suite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: benchmark: %v\n", err)
		return 1
	}

	// Save results.
	outPath := filepath.Join(outDir, fmt.Sprintf("bench-%d.json", time.Now().Unix()))
	if err := comparison.SaveBenchmarkResult(outPath, result); err != nil {
		fmt.Fprintf(os.Stderr, "ddx-agent-bench run: save: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(result.Summary, "", "  ")
		fmt.Println(string(data))
	} else {
		printSummaryTable(result)
		fmt.Printf("\nResults written to: %s\n", outPath)
	}

	return 0
}

// buildSuite converts corpus tasks and candidates into a BenchmarkSuite.
func buildSuite(tasks []CorpusTask, candidates []Candidate) *comparison.BenchmarkSuite {
	arms := make([]comparison.BenchmarkArm, 0, len(candidates))
	for _, c := range candidates {
		label := c.Harness
		if c.Provider != "" {
			label = c.Harness + "/" + c.Provider
		}
		if c.Model != "" {
			label += "/" + c.Model
		}
		arms = append(arms, comparison.BenchmarkArm{
			Label:   label,
			Harness: c.Harness,
			Model:   c.Model,
		})
	}

	prompts := make([]comparison.BenchmarkPrompt, 0, len(tasks))
	for _, t := range tasks {
		prompts = append(prompts, comparison.BenchmarkPrompt{
			ID:          t.ID,
			Name:        t.Description,
			Description: t.Description,
			Prompt:      t.Prompt,
			Tags:        t.Tags,
		})
	}

	return &comparison.BenchmarkSuite{
		Name:    "ddx-agent-bench",
		Version: "1",
		Arms:    arms,
		Prompts: prompts,
	}
}
