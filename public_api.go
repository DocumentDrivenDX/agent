package fizeau

import (
	agentcore "github.com/DocumentDrivenDX/fizeau/internal/core"
	"github.com/DocumentDrivenDX/fizeau/internal/reasoning"
)

type Tool = agentcore.Tool

type Reasoning = reasoning.Reasoning

const (
	ReasoningAuto    = reasoning.ReasoningAuto
	ReasoningOff     = reasoning.ReasoningOff
	ReasoningLow     = reasoning.ReasoningLow
	ReasoningMedium  = reasoning.ReasoningMedium
	ReasoningHigh    = reasoning.ReasoningHigh
	ReasoningMinimal = reasoning.ReasoningMinimal
	ReasoningXHigh   = reasoning.ReasoningXHigh
	ReasoningMax     = reasoning.ReasoningMax
)

func ReasoningTokens(n int) Reasoning {
	return reasoning.ReasoningTokens(n)
}
