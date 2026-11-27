//go:build windows

package firewall

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modShell32        = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// CanRelaunchElevated reports whether this platform supports one-click UAC relaunch.
func CanRelaunchElevated() bool {
	return true
}

// RelaunchElevated starts a new copy of the current executable with the Windows
// "runas" verb (triggering the UAC administrator prompt) and exits the current
// process on success.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %w", err)
	}
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	argsPtr, err := syscall.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	cwdPtr, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}

	const swShowNormal = 1
	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		uintptr(unsafe.Pointer(cwdPtr)),
		uintptr(swShowNormal),
	)
	// ShellExecuteW returns a value > 32 on success.
	if ret <= 32 {
		return fmt.Errorf("管理员提权启动失败 (code=%d): %v", ret, callErr)
	}
	os.Exit(0)
	return nil
}
