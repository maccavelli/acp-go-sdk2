package agentproc_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
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

// An agent written in Go, run in this process: Serve is what the agent's own
// main runs over its standard input and output, here over pipes, and the
// Process is the one a child gets. Close's first step ends Serve's input, and
// acp.ServeAgent returns at its end. A panic in Serve is not recovered; an
// agent that must not take its client down with it runs as a child.
func ExampleInProcess() {
	agent := &acptest.ScriptAgent{} // the agent's acp.Agent
	p, err := (&agentproc.InProcess{Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent, in, out)
	}}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := p.Conn().Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("protocol version", resp.ProtocolVersion)
	fmt.Println("close:", p.Close())
	// Output:
	// protocol version 1
	// close: <nil>
}

// An agent closed within a bound. Close waits for the client's handlers,
// and a handler that ignores its context would hold it; CloseContext is
// Close bounded by ctx. Should the bound pass, the steps left end in the
// kill, and the error says what still ran, with errors.Is.
func ExampleProcess_CloseContext() {
	agent := &acptest.ScriptAgent{}
	p, err := (&agentproc.InProcess{Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent, in, out)
	}}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	// context.WithTimeoutCause gives the error a cause of the caller's own.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = p.CloseContext(ctx)
	if errors.Is(err, acp.ErrHandlersRunning) {
		log.Printf("a client handler still runs; Wait returns once it has: %v", err)
	}
	fmt.Println("close:", err)
	// Output:
	// close: <nil>
}

// Both ways of the wire recorded, one record a frame, as a test reads them
// back with acptest.ReadRecords, validates them with acptest.ValidateGolden,
// and decodes them with acptest.Decode. Close drains the taps, so the
// transcript is whole once it returns.
func ExampleTranscript() {
	var record bytes.Buffer
	tr := agentproc.NewTranscript(&record)
	agent := &acptest.ScriptAgent{}
	p, err := (&agentproc.InProcess{
		Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
			return acp.ServeAgent(ctx, agent, in, out)
		},
		InputTap:  tr.Input(),
		OutputTap: tr.Output(),
	}).Start(client{})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := p.Conn().Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		log.Fatal(err)
	}
	if err := errors.Join(p.Close(), tr.Close()); err != nil {
		log.Fatal(err)
	}
	records, err := acptest.ReadRecords(&record)
	if err != nil {
		log.Fatal(err)
	}
	sent, err := acptest.Decode[acp.InitializeRequest](records, acptest.Sent, acp.AgentMethodInitialize)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(records), "records; the client asked for protocol version", sent[0].ProtocolVersion)
	// Output:
	// 2 records; the client asked for protocol version 1
}
