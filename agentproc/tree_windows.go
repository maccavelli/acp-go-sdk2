//go:build windows

package agentproc

import (
	"errors"
	"fmt"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hasGroups reports that there is no process group to signal here: Windows
// does not implement sending an interrupt
// (0027-MADR-agent-process-helper.md, S3).
const hasGroups = false

// hasTree reports that stopping the agent ends the processes it started.
const hasTree = true

// prepare starts the agent suspended, so that it runs nothing until attach
// has put it in its job, keeping the caller's other creation flags; it
// refuses, starting nothing, a Cmd the caller would start suspended itself,
// since attach resumes it (0030-MADR-record-initialize-on-arrival.md,
// "Amendment 2026-10-21, Windows: a process the agent starts before its job
// assignment", J4 and question 12).
func prepare(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_SUSPENDED != 0 {
		return errors.New("agentproc: Command.Cmd.SysProcAttr.CreationFlags has CREATE_SUSPENDED; Start starts the agent suspended itself")
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	return nil
}

// attachDelay, zero but in a test, is how long attach waits before it
// assigns the agent to its job: a test makes the assignment late with it, as
// a slow client would, which only the suspended start keeps from mattering
// (the amendment above, question 13).
var attachDelay atomic.Int64

// tree is the job object the agent and the processes it starts run in.
type tree struct {
	cmd *exec.Cmd
	job windows.Handle
}

// attach creates a job object that ends its processes when its last handle
// closes, should this process die too (P17, P18), assigns the agent to it
// through the handle os.Process.WithHandle gives, which meets no reused
// process ID (S5), and then resumes the agent, which prepare started
// suspended: a process the agent starts is in the job however late the
// assignment (0030-MADR-record-initialize-on-arrival.md, "Amendment
// 2026-10-21, Windows: a process the agent starts before its job
// assignment"). The job allows breakaway: a process started with
// CREATE_BREAKAWAY_FROM_JOB, which a job without the limit refuses, is
// outside it, as a process that starts a session of its own is outside the
// process group on Unix (0030-MADR-record-initialize-on-arrival.md, the
// amendment of 2026-10-21, W3 and W4).
func attach(cmd *exec.Cmd) (*tree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating the agent's job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil { //nolint:gosec // G103: the structure Windows reads, as x/sys lays it out.
		return nil, errors.Join(fmt.Errorf("limiting the agent's job object: %w", err), windows.CloseHandle(job))
	}
	if d := attachDelay.Load(); d > 0 {
		time.Sleep(time.Duration(d))
	}
	var aerr error
	if err := cmd.Process.WithHandle(func(h uintptr) { aerr = windows.AssignProcessToJobObject(job, windows.Handle(h)) }); err != nil {
		aerr = err
	}
	if aerr != nil {
		return nil, errors.Join(fmt.Errorf("assigning the agent to its job object: %w", aerr), windows.CloseHandle(job))
	}
	if err := resume(uint32(cmd.Process.Pid)); err != nil { //nolint:gosec // G115: a Windows process ID is a DWORD.
		return nil, errors.Join(fmt.Errorf("resuming the agent: %w", err), windows.CloseHandle(job))
	}
	return &tree{cmd: cmd, job: job}, nil
}

// resume resumes the threads of process pid, the one thread of a process
// created suspended: os/exec closes the handle of its main thread, so they
// are found by a thread snapshot and opened by their IDs (J5).
func resume(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("listing its threads: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a snapshot handle: nothing to report.
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := 0
	for err = windows.Thread32First(snap, &entry); err == nil; err = windows.Thread32Next(snap, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		th, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return fmt.Errorf("opening thread %d: %w", entry.ThreadID, err)
		}
		_, rerr := windows.ResumeThread(th)
		cerr := windows.CloseHandle(th)
		if rerr != nil {
			return fmt.Errorf("resuming thread %d: %w", entry.ThreadID, rerr)
		}
		if cerr != nil {
			return fmt.Errorf("closing thread %d: %w", entry.ThreadID, cerr)
		}
		resumed++
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("listing its threads: %w", err)
	}
	if resumed == 0 {
		return errors.New("no thread of it to resume")
	}
	return nil
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
