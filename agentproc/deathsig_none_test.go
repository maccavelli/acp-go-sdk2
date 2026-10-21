//go:build unix && !linux && !freebsd

package agentproc

import "os/exec"

// setInertDeathSignal does nothing: there is no Pdeathsig here.
func setInertDeathSignal(*exec.Cmd) {}
