package agentproc_test

import (
	"context"
	"log"
	"os"
	"os/exec"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/agentproc"
)

// client answers the agent; a real one implements every acp.Client method.
type client struct{ acp.Client }

func (client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

// An agent started as a child process, used, and stopped with the processes
// it started: Close closes its input, waits, signals, and kills, each step
// bounded by the grace.
func ExampleCommand() {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "my-agent", "--acp")
	cmd.Stderr = os.Stderr // the agent's log
	p, err := (&agentproc.Command{Cmd: cmd}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			log.Printf("agent: %v", err)
		}
	}()
	if _, err := p.Conn().Initialize(ctx, acp.InitializeRequest{}); err != nil {
		log.Printf("initialize: %v", err)
	}
}
