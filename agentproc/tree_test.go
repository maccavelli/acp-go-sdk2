package agentproc

import "github.com/maccavelli/acp-go-sdk2/internal/proctree"

// hasGroups reports that the agent runs in a process group Close can signal.
const hasGroups = proctree.HasGroups

// hasTree reports that stopping the agent ends the processes it started.
const hasTree = proctree.HasTree
