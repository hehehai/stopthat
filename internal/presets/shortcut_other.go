//go:build !windows

package presets

// ResolveShortcut returns p unchanged on non-Windows platforms.
func ResolveShortcut(p string) string {
	return p
}
