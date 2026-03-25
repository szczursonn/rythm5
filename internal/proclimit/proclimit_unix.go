//go:build linux || darwin

package proclimit

import "syscall"

func (cpup CPUPriority) asNiceValue() int {
	switch cpup {
	case CPUPriorityLow:
		return 10
	default:
		return 0
	}
}

func ApplyCPUPriority(pid int, priority CPUPriority) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority.asNiceValue())
}
