//go:build unix

package clienthost

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// TestFilesFIFOSwap requires ReadTextFile never to block on a path that a
// process flips between a regular file and a FIFO: it checks the file it
// opened, and opens without waiting for a FIFO's other end. Checking the path
// before opening it left a window in which the open blocked, with no context
// to end it (0049-PLAN-subpackage-fixes.md, phase 2, P4).
func TestFilesFIFOSwap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "f")
	reg := filepath.Join(dir, "reg")
	fifo := filepath.Join(dir, "fifo")
	var stop atomic.Bool
	flipped := make(chan struct{})
	go func() {
		defer close(flipped)
		for !stop.Load() {
			_ = os.WriteFile(reg, []byte("x\n"), 0o600)
			_ = os.Rename(reg, target)
			_ = unix.Mkfifo(fifo, 0o600)
			_ = os.Rename(fifo, target)
		}
	}()
	const tries = 50000
	deadline := time.Now().Add(2 * time.Second)
	hung, ran := 0, 0
	for ; ran < tries && time.Now().Before(deadline) && hung < 3; ran++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = Files{}.ReadTextFile(t.Context(), acp.ReadTextFileRequest{SessionId: "s", Path: target})
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			hung++
		}
	}
	stop.Store(true)
	<-flipped
	if hung > 0 {
		t.Errorf("%d of %d ReadTextFile calls on a path flipped to a FIFO were still blocked after 500ms", hung, ran)
	}
	// Release a blocked open, if any: a writer's open ends a reader's wait.
	for range hung + 2 {
		if f, err := os.OpenFile(target, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = f.Close()
		}
	}
}

// TestFilesRefuseFIFO requires both methods to refuse a FIFO no other process
// has open, at once, as not a regular file: a read's open returns without a
// writer, and a write's fails without a reader
// (0049-PLAN-subpackage-fixes.md, phase 2, P4).
func TestFilesRefuseFIFO(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"ReadTextFile": func() error {
			_, err := Files{}.ReadTextFile(t.Context(), acp.ReadTextFileRequest{SessionId: "s", Path: fifo})
			return err
		},
		"WriteTextFile": func() error {
			_, err := Files{}.WriteTextFile(t.Context(), acp.WriteTextFileRequest{SessionId: "s", Path: fifo, Content: "x"})
			return err
		},
	} {
		errs := make(chan error, 1)
		go func() { errs <- call() }()
		select {
		case err := <-errs:
			if err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Errorf("%s of a FIFO = %v, want a refusal that it is not a regular file", name, err)
			}
		case <-time.After(time.Second):
			t.Errorf("%s of a FIFO did not return within 1s", name)
			if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
				_ = f.Close()
			}
		}
	}
}
