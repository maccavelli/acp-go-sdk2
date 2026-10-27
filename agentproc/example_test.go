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
	cmd := exec.Command("my-agent", "--acp")
	// The agent's log.
	cmd.Stderr = os.Stderr
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

// The agent's output as frames, its lines, as the connection reads them: a
// line too long for the connection, or not JSON, included.
func ExampleLineTap() {
	cmd := exec.Command("my-agent", "--acp")
	cmd.Stderr = os.Stderr
	tap := agentproc.LineTap(func(frame []byte) { log.Printf("agent → client: %s", frame) })
	p, err := (&agentproc.Command{Cmd: cmd, OutputTap: tap}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	if err := p.Close(); err != nil {
		log.Printf("agent: %v", err)
	}
}

// The agent's output copied, byte for byte, to a file as the connection
// reads it: a record of the wire. The tap writes on a goroutine of its own,
// and DroppedTapBytes says whether it kept up.
func ExampleCommand_outputTap() {
	ctx := context.Background()
	f, err := os.Create("agent-output.jsonl")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("record: %v", err)
		}
	}()
	cmd := exec.Command("my-agent", "--acp")
	cmd.Stderr = os.Stderr
	p, err := (&agentproc.Command{Cmd: cmd, OutputTap: f}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			log.Printf("agent: %v", err)
		}
		if out, _ := p.DroppedTapBytes(); out > 0 {
			log.Printf("the record lacks the last %d bytes", out)
		}
	}()
	if _, err := p.Conn().Initialize(ctx, acp.InitializeRequest{}); err != nil {
		log.Printf("initialize: %v", err)
	}
}
