package proclimit

import "errors"

var ErrUnsupportedPlatform = errors.New("proclimit: unsupported platform")

type CPUPriority int

const (
	CPUPriorityUnset CPUPriority = iota
	CPUPriorityLow
	CPUPriorityNormal
)

type OOMKillerPriority int

const (
	OOMKillerPriorityUnset OOMKillerPriority = iota
	OOMKillerPriorityNormal
	OOMKillerPriorityAboveNormal
	OOMKillerPriorityHigh
)
