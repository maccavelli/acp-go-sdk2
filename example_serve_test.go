package acp_test

import (
	"context"
	"log"
	"os"
	"os/signal"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// An agent's main: ServeAgent serves the agent on its standard input and
// output until a client ends the input, or the interrupt ends ctx. Then a
// prompt still running is answered cancelled, as its handler reads its
// context's end, before the connection closes
// (0042-MADR-in-process-agent-runner.md, item 2).
func ExampleServeAgent() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var agent acp.Agent = &acptest.ScriptAgent{} // the agent's own acp.Agent
	if err := acp.ServeAgent(ctx, agent, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		log.Printf("serve: %v", err)
	}
}
