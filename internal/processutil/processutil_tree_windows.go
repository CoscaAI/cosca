//go:build windows

package processutil

import (
	"os/exec"
	"strconv"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// treeKiller terminates a command's whole process tree on Windows. The primary
// mechanism is a Job Object configured with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
// whose members are killed with TerminateJobObject. If the Job Object cannot be
// created or the process cannot be assigned (for example when the host process
// is already inside a job that forbids nesting, as on some CI runners), it
// falls back to `taskkill /T /F /PID`, which walks the parent/child tree by PID
// and also closes the assignment race for grandchildren spawned before the
// process reached the job.
type treeKiller struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	job      windows.Handle
	assigned bool
}

func newTreeKiller(cmd *exec.Cmd) *treeKiller {
	tk := &treeKiller{cmd: cmd}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		// Job Object unavailable: the taskkill fallback handles termination.
		return tk
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return tk
	}
	tk.job = job
	return tk
}

// attach binds the running process to the job. It must be called after the
// process has started. Best-effort: if assignment fails we stay in taskkill
// fallback mode. It is idempotent.
func (tk *treeKiller) attach() {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if tk.job == 0 || tk.assigned || tk.cmd.Process == nil {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(tk.cmd.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph)
	if err := windows.AssignProcessToJobObject(tk.job, ph); err == nil {
		tk.assigned = true
	}
}

func (tk *treeKiller) kill() error {
	tk.mu.Lock()
	job := tk.job
	assigned := tk.assigned
	pid := 0
	if tk.cmd.Process != nil {
		pid = tk.cmd.Process.Pid
	}
	tk.mu.Unlock()

	var err error
	if job != 0 && assigned {
		if werr := windows.TerminateJobObject(job, 1); werr != nil {
			err = werr
		}
	}
	// Fallback / race-closing sweep: taskkill terminates the whole tree by parent
	// PID, catching any grandchild that was spawned before the job assignment.
	if pid > 0 {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	return err
}

func (tk *treeKiller) release() {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if tk.job != 0 {
		windows.CloseHandle(tk.job)
		tk.job = 0
		tk.assigned = false
	}
}
