//go:build windows

package proclimit

import "golang.org/x/sys/windows"

const (
	belowNormalPriorityClass = 0x00004000
	normalPriorityClass      = 0x00000020
)

func (cpup CPUPriority) asPriorityClass() uint32 {
	switch cpup {
	case CPUPriorityLow:
		return belowNormalPriorityClass
	default:
		return normalPriorityClass
	}
}

func ApplyCPUPriority(pid int, priority CPUPriority) error {
	handle, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	return windows.SetPriorityClass(handle, priority.asPriorityClass())
}

func ApplyOOMKillerPriority(pid int, priority OOMKillerPriority) error {
	return ErrUnsupportedPlatform
}
