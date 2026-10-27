// Package proctree runs a process in a process group or job object of its
// own, so that ending it ends the processes it started: on Unix a process
// group whose ID is the process's, on Windows a job object that ends its
// processes when its last handle closes, and elsewhere the process alone.
// agentproc and clienthost share it
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, question 1;
// 0027-MADR-agent-process-helper.md).
//
// A caller calls Prepare on the Cmd before starting it, and Attach once it
// has started; Terminate, Kill and End then act on the Tree Attach returns.
package proctree

import "errors"

// ErrPgidSet is Prepare's refusal, on Unix, of a Cmd that would join a
// process group other processes share.
var ErrPgidSet = errors.New("proctree: SysProcAttr.Pgid is set; the process needs a process group of its own")

// ErrSuspended is Prepare's refusal, on Windows, of a Cmd the caller would
// start suspended itself, since Attach resumes it.
var ErrSuspended = errors.New("proctree: SysProcAttr.CreationFlags has CREATE_SUSPENDED; Prepare starts the process suspended itself")
