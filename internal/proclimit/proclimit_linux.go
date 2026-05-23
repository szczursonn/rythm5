//go:build linux

package proclimit

import (
	"fmt"
	"os"
)

func SetOOMScoreAdj(pid int, score int) error {
	return os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid), fmt.Appendf(nil, "%d\n", score), 0644)
}
