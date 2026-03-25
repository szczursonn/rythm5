//go:build !linux && !darwin && !windows

package proclimit

func ApplyOOMKillerPriority(pid int, priority OOMKillerPriority) error {
	return ErrUnsupportedPlatform
}

func ApplyCPUPriority(pid int, priority CPUPriority) error {
	return ErrUnsupportedPlatform
}
