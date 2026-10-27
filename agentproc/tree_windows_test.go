//go:build windows

package agentproc

import "github.com/maccavelli/acp-go-sdk2/internal/proctree"

// attachDelay is internal/proctree's AttachDelay, which the suspended-start
// test sets.
var attachDelay = &proctree.AttachDelay
