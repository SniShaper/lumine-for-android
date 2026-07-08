//go:build android && !cgo

package platform

import (
	"os/exec"
	"strings"
	"time"
)

func init() {
	output, err := exec.Command("getprop", "persist.sys.timezone").Output()
	if err != nil {
		return
	}
	timezone := strings.TrimSpace(string(output))
	if location, err := time.LoadLocation(timezone); err == nil {
		time.Local = location
	}
}
