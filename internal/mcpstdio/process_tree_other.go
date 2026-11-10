//go:build !unix && !windows

package mcpstdio

import "os/exec"

// processTree 在没有进程组或 Job Object 支持的平台上只能终止直接子进程。
type processTree struct {
	cmd *exec.Cmd
}

func newProcessTree(cmd *exec.Cmd) *processTree {
	return &processTree{cmd: cmd}
}

func (t *processTree) attach() {}

func (t *processTree) kill() error {
	if t.cmd.Process == nil {
		return nil
	}
	return t.cmd.Process.Kill()
}

func (t *processTree) release() {}
