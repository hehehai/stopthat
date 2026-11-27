//go:build !windows

package firewall

import "fmt"

// CanRelaunchElevated reports whether this platform supports one-click UAC relaunch.
func CanRelaunchElevated() bool {
	return false
}

// RelaunchElevated is only supported on Windows.
func RelaunchElevated() error {
	return fmt.Errorf("当前平台不支持 UAC 重启提权")
}
