package agent_test

import (
	"context"
	"testing"

	agent "github.com/DocumentDrivenDX/agent"
	"github.com/stretchr/testify/require"
)

func TestLegacyPublicAPIStillCompiles(t *testing.T) {
	var _ agent.Provider = (*externalMockProvider)(nil)
	var _ agent.Tool = oversizedOutputTool{}
	var compactor agent.Compactor = func(_ context.Context, messages []agent.Message, _ agent.Provider, _ []agent.ToolCallLog) ([]agent.Message, *agent.CompactionResult, error) {
		return messages, nil, nil
	}

	provider := &externalMockProvider{
		responses: []agent.Response{{Content: "legacy ok"}},
	}
	result, err := agent.Run(context.Background(), agent.Request{
		Prompt:    "legacy compile smoke",
		Provider:  provider,
		Compactor: compactor,
	})
	require.NoError(t, err)
	require.Equal(t, agent.StatusSuccess, result.Status)
	require.Equal(t, "legacy ok", result.Output)
}
