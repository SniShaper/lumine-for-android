//go:build android && arm64

package lumine

import (
	"os/exec"
	"strings"
	"time"
)

func init() {
	// On Android, the Go standard library does not read the system timezone
	// and defaults to UTC. To obtain the correct system timezone, we use the
	// `getprop persist.sys.timezone` command instead of cgo, which allows
	// the software to remain pure Go. This approach ensures that time.Local
	// is set according to the actual Android system timezone.
	cmd := exec.Command("getprop", "persist.sys.timezone")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	timezone := strings.TrimSpace(string(output))
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return
	}
	time.Local = location
}
