package clienthost

import acp "github.com/maccavelli/acp-go-sdk2"

// Refusals is acp.ClientRefusals one level deeper, so that a type of this
// package embedded beside it wins. acp.ClientRefusals declares the terminal
// and file-system methods Terminals and Files declare, and Go promotes
// neither of two methods of one name at one depth; through Refusals,
// ClientRefusals' are a level further down, and Terminals' and Files' are
// the client's
// (0041-PLAN-close-the-gaps-the-consumers-wait-on.md, phase 8).
type Refusals struct{ acp.ClientRefusals }
