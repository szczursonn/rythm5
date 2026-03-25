package proclimit

import "errors"

var ErrUnsupportedPlatform = errors.New("proclimit: unsupported platform")

type CPUPriority int

const (
	CPUPriorityLow CPUPriority = iota
	CPUPriorityNormal
)

type OOMKillerPriority int

const (
	OOMKillerPriorityNormal OOMKillerPriority = iota
	OOMKillerPriorityAboveNormal
	OOMKillerPriorityHigh
)
