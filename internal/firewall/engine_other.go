//go:build !windows && !darwin

package firewall

import "fmt"

// NewEngine returns an in-memory simulated engine on Linux / unsupported OSes
// so the app can run and be previewed during development.
func NewEngine() Engine {
	return NewMockEngine("linux", "模拟防火墙引擎（开发预览）")
}

// OpenSystemFirewallSettings is not supported on generic Linux builds.
func OpenSystemFirewallSettings() error {
	return fmt.Errorf("当前平台不支持自动打开系统防火墙设置")
}
