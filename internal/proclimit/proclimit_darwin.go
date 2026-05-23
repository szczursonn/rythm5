//go:build darwin

package proclimit

func SetOOMScoreAdj(pid int, score int) error {
	return ErrUnsupportedPlatform
}
