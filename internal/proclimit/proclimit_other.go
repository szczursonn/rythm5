//go:build !linux && !darwin

package proclimit

func SetNiceValue(pid int, value int) error {
	return ErrUnsupportedPlatform
}

func SetOOMScoreAdj(pid int, score int) error {
	return ErrUnsupportedPlatform
}
