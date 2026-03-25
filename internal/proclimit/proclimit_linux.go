//go:build linux

package proclimit

import (
	"fmt"
	"os"
)

func (oomp OOMKillerPriority) asOOMScoreAdj() int {
	switch oomp {
	case OOMKillerPriorityAboveNormal:
		return 500
	case OOMKillerPriorityHigh:
		return 1000
	default:
		return 0
	}
}

func ApplyOOMKillerPriority(pid int, priority OOMKillerPriority) error {
	return os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid), fmt.Appendf(nil, "%d\n", priority.asOOMScoreAdj()), 0644)
}
