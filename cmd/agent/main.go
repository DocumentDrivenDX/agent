// Command ddx-agent is a standalone CLI that wraps the agent library.
package main

import (
	"os"

	"github.com/DocumentDrivenDX/agent/agentcli"
)

// Version info set via -ldflags.
var (
	Version   = "dev"
	BuildTime = ""
	GitCommit = ""
)

func main() {
	os.Exit(agentcli.Run(agentcli.Options{
		Args:      os.Args[1:],
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}))
}
