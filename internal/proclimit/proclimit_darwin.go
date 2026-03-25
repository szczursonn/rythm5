//go:build darwin

package proclimit

func ApplyOOMKillerPriority(pid int, priority OOMKillerPriority) error {
	return ErrUnsupportedPlatform
}
