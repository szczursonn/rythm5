//go:build linux || darwin

package proclimit

import "syscall"

func SetNiceValue(pid int, value int) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, value)
}
