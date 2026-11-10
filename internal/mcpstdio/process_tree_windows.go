//go:build windows

package mcpstdio

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processTree 把 mcp_stdio 子进程放入 KILL_ON_JOB_CLOSE 的 Job Object，后代默认继承该 job。
// job 句柄关闭时（包括 onesearch 异常退出由系统回收句柄）整棵进程树都会被终止。
type processTree struct {
	cmd *exec.Cmd
	mu  sync.Mutex
	job windows.Handle
}

// newProcessTree 必须在 Start 前调用；ctx 取消时由 cmd.Cancel 终止整棵进程树。
func newProcessTree(cmd *exec.Cmd) *processTree {
	tree := &processTree{cmd: cmd}
	cmd.Cancel = tree.kill
	return tree
}

// attach 在 Start 后把子进程加入 job。子进程在 Start 与 attach 之间极短窗口内派生的后代不会进入 job；
// 任一步骤失败时有意退化为只终止直接子进程（即修复前的行为），不让进程树管理失败影响工具调用本身。
func (t *processTree) attach() {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(t.cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return
	}
	err = windows.AssignProcessToJobObject(job, process)
	_ = windows.CloseHandle(process)
	if err != nil {
		_ = windows.CloseHandle(job)
		return
	}
	t.mu.Lock()
	t.job = job
	t.mu.Unlock()
}

// kill 终止 job 内全部进程；未成功建立 job 时只终止直接子进程。
func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.job != 0 {
		if err := windows.TerminateJobObject(t.job, 1); err == nil {
			return nil
		}
	}
	if t.cmd.Process == nil {
		return nil
	}
	return t.cmd.Process.Kill()
}

// release 关闭 job 句柄；KILL_ON_JOB_CLOSE 会终止直接子进程退出后仍残留的后代。
func (t *processTree) release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.job != 0 {
		_ = windows.CloseHandle(t.job)
		t.job = 0
	}
}
