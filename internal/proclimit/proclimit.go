package proclimit

import (
	"errors"
)

const errPrefix = "proclimit: "

var ErrUnsupportedPlatform = errors.New(errPrefix + "unsupported platform")
