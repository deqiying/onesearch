//go:build unix

package mcpstdio

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// processTree 让 mcp_stdio 子进程成为独立进程组的组长，后代默认继承该进程组。
// 取舍：终端 Ctrl+C 不再直接送达该进程组；onesearch 退出后 provider 依赖 stdin EOF
// 自行退出，这与 MCP stdio 的关闭约定一致。
type processTree struct {
	cmd *exec.Cmd
}

// newProcessTree 必须在 Start 前调用；ctx 取消时由 cmd.Cancel 终止整个进程组。
func newProcessTree(cmd *exec.Cmd) *processTree {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	tree := &processTree{cmd: cmd}
	cmd.Cancel = tree.kill
	return tree
}

func (t *processTree) attach() {}

// kill 向整个进程组发送 SIGKILL；进程组已不存在时返回 os.ErrProcessDone。
func (t *processTree) kill() error {
	if t.cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// release 在直接子进程被回收后清理仍存活的同组后代；组内已无进程时 ESRCH 属于正常结果。
func (t *processTree) release() {
	_ = t.kill()
}
