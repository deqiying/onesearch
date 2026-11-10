//go:build unix

package mcpstdio

import (
	"errors"
	"syscall"
)

// processAlive 以 signal 0 探测；ESRCH 表示进程已不存在，EPERM 仍视为存活。
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}

func killProcess(pid int) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
