package presets

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// TargetKind classifies a discovered executable inside a suite or custom folder.
type TargetKind string

const (
	KindMain       TargetKind = "main"
	KindBackground TargetKind = "background"
	KindHelper     TargetKind = "helper"
)

// Label returns a human-readable Chinese label for the target kind.
func (k TargetKind) Label() string {
	switch k {
	case KindMain:
		return "主程序"
	case KindBackground:
		return "后台组件"
	default:
		return "辅助程序"
	}
}

// Target represents a single executable discovered on disk that can be managed
// by the firewall engine.
type Target struct {
	ID       string     `json:"id"`
	PresetID string     `json:"preset_id"` // "adobe", "custom", etc.
	Name     string     `json:"name"`      // friendly display name
	ExecName string     `json:"exec_name"` // base file name
	Path     string     `json:"path"`      // normalized absolute executable path
	Kind     TargetKind `json:"kind"`
	Note     string     `json:"note,omitempty"`
}

// Preset describes a built-in application suite profile with known install
// locations and executable classification hints.
type Preset struct {
	ID          string
	Name        string
	ShortName   string
	Description string
	WinRoots    []string
	MacRoots    []string
	// MacTopPrefix limits top-level entries inside shared directories like /Applications.
	MacAppPrefix string
}

// Builtin returns all built-in application suite presets.
func Builtin() []Preset {
	return []Preset{
		AdobePreset(),
	}
}

// AdobePreset returns the built-in Adobe Creative Cloud suite preset.
func AdobePreset() Preset {
	home, _ := os.UserHomeDir()
	winRoots := dedupeStrings([]string{
		envPath("ProgramFiles", "Adobe"),
		envPath("ProgramFiles(x86)", "Adobe"),
		envPath("CommonProgramFiles", "Adobe"),
		envPath("CommonProgramFiles(x86)", "Adobe"),
		`C:\Program Files\Adobe`,
		`C:\Program Files (x86)\Adobe`,
		`C:\Program Files\Common Files\Adobe`,
		`C:\Program Files (x86)\Common Files\Adobe`,
	})
	macRoots := dedupeStrings([]string{
		"/Applications",
		"/Library/Application Support/Adobe",
		filepath.Join(home, "Library", "Application Support", "Adobe"),
	})
	return Preset{
		ID:           "adobe",
		Name:         "Adobe Creative Cloud",
		ShortName:    "Adobe 套件",
		Description:  "自动扫描本机 Photoshop、Illustrator、Premiere Pro、After Effects、Lightroom 等主程序及关联后台组件，统一管理出站/入站联网权限。",
		WinRoots:     winRoots,
		MacRoots:     macRoots,
		MacAppPrefix: "adobe",
	}
}

// DefaultRoots returns the default scan directories for the current OS.
func (p Preset) DefaultRoots() []string {
	switch runtime.GOOS {
	case "windows":
		return p.WinRoots
	case "darwin":
		return p.MacRoots
	default:
		// Useful for Linux development/testing if paths exist.
		return nil
	}
}

func envPath(envKey string, sub string) string {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return ""
	}
	return filepath.Join(v, sub)
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

type knownApp struct {
	match string
	name  string
	kind  TargetKind
	note  string
}

var adobeKnown = []knownApp{
	// Main creative applications
	{match: "photoshop", name: "Adobe Photoshop", kind: KindMain},
	{match: "illustrator", name: "Adobe Illustrator", kind: KindMain},
	{match: "afterfx", name: "Adobe After Effects", kind: KindMain},
	{match: "after effects", name: "Adobe After Effects", kind: KindMain},
	{match: "adobe premiere pro", name: "Adobe Premiere Pro", kind: KindMain},
	{match: "premiere", name: "Adobe Premiere Pro", kind: KindMain},
	{match: "lightroom", name: "Adobe Lightroom", kind: KindMain},
	{match: "indesign", name: "Adobe InDesign", kind: KindMain},
	{match: "audition", name: "Adobe Audition", kind: KindMain},
	{match: "adobe media encoder", name: "Adobe Media Encoder", kind: KindMain},
	{match: "animate", name: "Adobe Animate", kind: KindMain},
	{match: "bridge", name: "Adobe Bridge", kind: KindMain},
	{match: "acrobat", name: "Adobe Acrobat", kind: KindMain},
	{match: "dreamweaver", name: "Adobe Dreamweaver", kind: KindMain},
	{match: "incopy", name: "Adobe InCopy", kind: KindMain},
	{match: "dimension", name: "Adobe Dimension", kind: KindMain},
	{match: "character animator", name: "Adobe Character Animator", kind: KindMain},
	{match: "xd", name: "Adobe XD", kind: KindMain},

	// Background / licensing / telemetry / sync components
	{match: "adobegcclient", name: "AdobeGCClient", kind: KindBackground, note: "后台联网组件"},
	{match: "agsservice", name: "AGSService", kind: KindBackground, note: "后台联网服务"},
	{match: "agmservice", name: "AGMService", kind: KindBackground, note: "后台联网服务"},
	{match: "agcinvokerutility", name: "AGCInvokerUtility", kind: KindBackground, note: "后台调用组件"},
	{match: "adobeipcbroker", name: "AdobeIPCBroker", kind: KindBackground, note: "进程通信组件"},
	{match: "ccxprocess", name: "CCXProcess", kind: KindBackground, note: "Creative Cloud 后台进程"},
	{match: "coresync", name: "CoreSync", kind: KindBackground, note: "云同步后台进程"},
	{match: "creative cloud", name: "Creative Cloud", kind: KindBackground, note: "桌面管理组件"},
	{match: "adobe desktop service", name: "Adobe Desktop Service", kind: KindBackground, note: "桌面后台服务"},
	{match: "adobe update", name: "Adobe Update", kind: KindBackground, note: "更新检查组件"},
	{match: "cclibrary", name: "CCLibrary", kind: KindBackground, note: "云库同步进程"},
	{match: "adobenotificationclient", name: "AdobeNotificationClient", kind: KindBackground, note: "通知后台进程"},
	{match: "crlogtransport", name: "CRLogTransport", kind: KindBackground, note: "日志上报组件"},
	{match: "logtransport", name: "LogTransport", kind: KindBackground, note: "日志上报组件"},
	{match: "crash", name: "Crash Reporter", kind: KindBackground, note: "崩溃上报组件"},
}

// ClassifyAdobe determines display name, kind, and note for an Adobe executable.
func ClassifyAdobe(execName string, fullPath string) (name string, kind TargetKind, note string) {
	base := strings.TrimSuffix(execName, filepath.Ext(execName))
	lowerBase := strings.ToLower(base)
	lowerPath := strings.ToLower(fullPath)

	for _, item := range adobeKnown {
		if item.match == "xd" {
			if lowerBase == "xd" || lowerBase == "adobe xd" {
				return item.name, item.kind, item.note
			}
			continue
		}
		if strings.Contains(lowerBase, item.match) {
			return item.name, item.kind, item.note
		}
	}

	if strings.Contains(lowerPath, "adobeclient") ||
		strings.Contains(lowerPath, "adobegcclient") ||
		strings.Contains(lowerPath, "common files") ||
		strings.Contains(lowerPath, "application support") {
		return base, KindBackground, "关联后台组件"
	}
	return base, KindHelper, ""
}
