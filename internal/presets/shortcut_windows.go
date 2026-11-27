//go:build windows

package presets

import (
	"bytes"
	"os/exec"
	"strings"
	"syscall"
)

// ResolveShortcut resolves a Windows .lnk shortcut file to its target executable path.
// If p is not a .lnk file or resolution fails, p is returned unchanged.
func ResolveShortcut(p string) string {
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(p)), ".lnk") {
		return p
	}
	escaped := strings.ReplaceAll(p, "'", "''")
	script := "$s = (New-Object -ComObject WScript.Shell).CreateShortcut('" + escaped + "'); $s.TargetPath"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return p
	}
	target := strings.TrimSpace(out.String())
	if target == "" {
		return p
	}
	return target
}
