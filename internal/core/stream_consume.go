package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DefaultReasoningByteLimit is the default maximum number of bytes of pure
// reasoning_content allowed before the stream is aborted with
// ErrReasoningOverflow. Only fires when no content or tool_call delta has been
// seen yet. Configurable via config.yaml reasoning_byte_limit; 0 = unlimited.
const DefaultReasoningByteLimit = 256 * 1024 // 256KB

// DefaultReasoningStallTimeout is the default maximum duration that only
// reasoning_content deltas may arrive before the stream is aborted with
// ErrReasoningStall. Configurable via config.yaml reasoning_stall_timeout;
// 0 = unlimited.
const DefaultReasoningStallTimeout = 300 * time.Second

// streamThresholds holds the configurable reasoning-loop detection thresholds
// passed into consumeStream.
type streamThresholds struct {
	reasoningByteLimit    int
	reasoningStallTimeout time.Duration
	modelName             string // included in error messages
}

// consumeStream reads from a StreamingProvider's channel, emits delta events,
// and assembles a complete Response. The thresholds parameter controls
// reasoning-loop detection limits; use streamThresholds{} for defaults.
func consumeStream(
	ctx context.Context,
	sp StreamingProvider,
	messages []Message,
	tools []ToolDef,
	opts Options,
	callback EventCallback,
	sessionID string,
	streamStart time.Time,
	seq *int,
	thresholds streamThresholds,
) (Response, error) {
	ch, err := sp.ChatStream(ctx, messages, tools, opts)
	if err != nil {
		return Response{}, err
	}

	var resp Response
	var contentBuf strings.Builder
	var firstOutputAt time.Time
	var lastOutputAt time.Time

	// 0 means unlimited for both thresholds (same pattern as MaxIterations).
	byteLimit := thresholds.reasoningByteLimit
	stallTimeout := thresholds.reasoningStallTimeout

	// Runaway reasoning loop detection.
	// These track state only while the model is in pure-reasoning mode (no
	// content or tool_call deltas seen yet for this response).
	var reasoningBytes int
	var nonReasoningSeen bool
	reasoningStallStart := time.Now()

	// Track tool call assembly — deltas arrive as fragments
	type toolCallState struct {
		ID      string
		Name    string
		ArgsBuf strings.Builder
	}
	toolCalls := make(map[string]*toolCallState)
	var toolCallOrder []string

	for delta := range ch {
		arrivalAt := delta.ArrivedAt
		if arrivalAt.IsZero() {
			arrivalAt = time.Now()
		}

		// Emit delta event
		if callback != nil {
			emitCallback(callback, Event{
				SessionID: sessionID,
				Seq:       *seq,
				Type:      EventLLMDelta,
				Timestamp: arrivalAt.UTC(),
				Data:      mustMarshal(delta),
			})
			*seq++
		}

		// Accumulate content
		if delta.Content != "" {
			contentBuf.WriteString(delta.Content)
		}

		// Accumulate tool call fragments
		if delta.ToolCallID != "" {
			tc, exists := toolCalls[delta.ToolCallID]
			if !exists {
				tc = &toolCallState{ID: delta.ToolCallID}
				toolCalls[delta.ToolCallID] = tc
				toolCallOrder = append(toolCallOrder, delta.ToolCallID)
			}
			if delta.ToolCallName != "" {
				tc.Name = delta.ToolCallName
			}
			if delta.ToolCallArgs != "" {
				tc.ArgsBuf.WriteString(delta.ToolCallArgs)
			}
		}

		// Capture model and finish reason from final delta
		if delta.Model != "" {
			resp.Model = delta.Model
		}
		if delta.Attempt != nil {
			attempt := *delta.Attempt
			resp.Attempt = &attempt
		}
		if delta.FinishReason != "" {
			resp.FinishReason = delta.FinishReason
		}
		if delta.Usage != nil {
			resp.Usage.Add(*delta.Usage)
		}

		if delta.Err != nil {
			return resp, delta.Err
		}

		// Runaway reasoning detection — only active while no content or tool_call
		// deltas have arrived yet for this response.
		if !nonReasoningSeen {
			if streamDeltaHasOutput(delta) {
				// First real output: disable reasoning loop checks for this response.
				nonReasoningSeen = true
			} else if delta.ReasoningContent != "" {
				reasoningBytes += len(delta.ReasoningContent)
				// 0 means unlimited (no limit).
				if byteLimit > 0 && reasoningBytes > byteLimit {
					return resp, reasoningOverflowError(thresholds.modelName, byteLimit, reasoningBytes)
				}
				if stallTimeout > 0 && time.Since(reasoningStallStart) > stallTimeout {
					return resp, reasoningStallError(thresholds.modelName, stallTimeout)
				}
			}
		}

		if streamDeltaHasOutput(delta) {
			if firstOutputAt.IsZero() {
				firstOutputAt = arrivalAt
			}
			lastOutputAt = arrivalAt
		}

		if delta.Done {
			break
		}
	}

	resp.Content = contentBuf.String()
	resp.Usage.Total = resp.Usage.Input + resp.Usage.Output

	if !firstOutputAt.IsZero() {
		if resp.Attempt == nil {
			resp.Attempt = &AttemptMetadata{}
		}
		if resp.Attempt.Timing == nil {
			resp.Attempt.Timing = &TimingBreakdown{}
		}
		firstToken := firstOutputAt.Sub(streamStart)
		generation := lastOutputAt.Sub(firstOutputAt)
		resp.Attempt.Timing.FirstToken = &firstToken
		resp.Attempt.Timing.Generation = &generation
	}

	// Assemble tool calls from fragments
	for _, id := range toolCallOrder {
		tc := toolCalls[id]
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: json.RawMessage(tc.ArgsBuf.String()),
		})
	}

	return resp, nil
}

func streamDeltaHasOutput(delta StreamDelta) bool {
	return delta.Content != "" ||
		delta.ToolCallID != "" ||
		delta.ToolCallName != "" ||
		delta.ToolCallArgs != ""
}

// reasoningOverflowError wraps ErrReasoningOverflow with model name and threshold.
func reasoningOverflowError(model string, limit, actual int) error {
	if model == "" {
		model = "unknown"
	}
	return fmt.Errorf("%w (model=%s, limit=%dKB, actual=%dKB)",
		ErrReasoningOverflow, model, limit/1024, actual/1024)
}

// reasoningStallError wraps ErrReasoningStall with model name and threshold.
func reasoningStallError(model string, timeout time.Duration) error {
	if model == "" {
		model = "unknown"
	}
	return fmt.Errorf("%w (model=%s, timeout=%s)", ErrReasoningStall, model, timeout)
}
