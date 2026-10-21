//go:build windows

package agentproc

import (
	"errors"
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hasGroups reports that there is no process group to signal here: Windows
// does not implement sending an interrupt
// (0027-MADR-agent-process-helper.md, S3).
const hasGroups = false

// hasTree reports that stopping the agent ends the processes it started.
const hasTree = true

// prepare has nothing to set before the agent starts: os/exec cannot start
// it suspended, so attach puts it in its job once it runs (S7).
func prepare(*exec.Cmd) error { return nil }

// tree is the job object the agent and the processes it starts run in.
type tree struct {
	cmd *exec.Cmd
	job windows.Handle
}

// attach creates a job object that ends its processes when its last handle
// closes, should this process die too (P17, P18), and assigns the agent to
// it through the handle os.Process.WithHandle gives, which meets no reused
// process ID (S5). A process the agent starts before the assignment is
// outside the job (S7).
func attach(cmd *exec.Cmd) (*tree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating the agent's job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil { //nolint:gosec // G103: the structure Windows reads, as x/sys lays it out.
		return nil, errors.Join(fmt.Errorf("limiting the agent's job object: %w", err), windows.CloseHandle(job))
	}
	var aerr error
	if err := cmd.Process.WithHandle(func(h uintptr) { aerr = windows.AssignProcessToJobObject(job, windows.Handle(h)) }); err != nil {
		aerr = err
	}
	if aerr != nil {
		return nil, errors.Join(fmt.Errorf("assigning the agent to its job object: %w", aerr), windows.CloseHandle(job))
	}
	return &tree{cmd: cmd, job: job}, nil
}

// terminate has no signal to send, so Close goes from the agent's input to
// the kill.
func (*tree) terminate() error { return errors.ErrUnsupported }

// kill ends the job, the agent and every process in it, with exit status 1,
// as Process.Kill reports on Windows (P6); Process.Kill is the fallback.
func (t *tree) kill() error {
	if err := windows.TerminateJobObject(t.job, 1); err != nil {
		return errors.Join(err, t.cmd.Process.Kill())
	}
	return nil
}

// end ends what is left of the job once the agent has exited, and closes its
// handle.
func (t *tree) end() {
	_ = windows.TerminateJobObject(t.job, 1) //nolint:errcheck // an emptied job is the common case.
	_ = windows.CloseHandle(t.job)           //nolint:errcheck // nothing is left to do with it.
}
