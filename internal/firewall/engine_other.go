//go:build !windows && !darwin

package firewall

// NewEngine returns an in-memory simulated engine on Linux / unsupported OSes
// so the app can run and be previewed during development.
func NewEngine() Engine {
	return NewMockEngine("linux", "模拟防火墙引擎（开发预览）")
}
