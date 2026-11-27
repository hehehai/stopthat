package presets

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const maxScanDepth = 5

// ScanOptions controls how directories are scanned for executables.
type ScanOptions struct {
	ExtraRoots []string
	OS         string // defaults to runtime.GOOS; overridable in tests
}

// ScanPreset discovers installed executables belonging to a preset.
func ScanPreset(p Preset, opts ScanOptions) []Target {
	goos := opts.OS
	if goos == "" {
		goos = runtime.GOOS
	}
	roots := append([]string{}, p.DefaultRoots()...)
	roots = append(roots, opts.ExtraRoots...)
	roots = dedupeStrings(roots)

	seen := make(map[string]struct{})
	var targets []Target
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if t, ok := targetFromFile(p.ID, root, goos); ok {
				addUnique(&targets, seen, t)
			}
			continue
		}
		// On macOS, /Applications contains all apps; only descend into entries matching MacAppPrefix.
		if goos == "darwin" && filepath.Clean(root) == "/Applications" && p.MacAppPrefix != "" {
			entries, err := os.ReadDir(root)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !strings.Contains(strings.ToLower(e.Name()), strings.ToLower(p.MacAppPrefix)) {
					continue
				}
				sub := filepath.Join(root, e.Name())
				for _, t := range scanPath(p.ID, sub, goos) {
					addUnique(&targets, seen, t)
				}
			}
			continue
		}
		for _, t := range scanPath(p.ID, root, goos) {
			addUnique(&targets, seen, t)
		}
	}

	sortTargets(targets)
	return targets
}

// ScanCustomPath scans a user-supplied executable file, .app bundle, or directory.
func ScanCustomPath(path string, goos string) []Target {
	if goos == "" {
		goos = runtime.GOOS
	}
	path = strings.Trim(strings.TrimSpace(path), `"'`)
	if path == "" {
		return nil
	}
	targets := scanPath("custom", path, goos)
	sortTargets(targets)
	return targets
}

func scanPath(presetID string, root string, goos string) []Target {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		if t, ok := targetFromFile(presetID, root, goos); ok {
			return []Target{t}
		}
		return nil
	}

	// macOS .app bundle selected directly.
	if goos == "darwin" && strings.HasSuffix(strings.ToLower(root), ".app") {
		return scanMacAppBundle(presetID, root)
	}

	var out []Target
	seen := make(map[string]struct{})
	rootDepth := pathDepth(root)

	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && pathDepth(p)-rootDepth > maxScanDepth {
				return filepath.SkipDir
			}
			lower := strings.ToLower(d.Name())
			if lower == "node_modules" || lower == "cache" || lower == "caches" || lower == "logs" || lower == "locales" {
				return filepath.SkipDir
			}
			if goos == "darwin" && strings.HasSuffix(lower, ".app") {
				for _, t := range scanMacAppBundle(presetID, p) {
					addUnique(&out, seen, t)
				}
				return filepath.SkipDir
			}
			return nil
		}

		if t, ok := targetFromFile(presetID, p, goos); ok {
			addUnique(&out, seen, t)
		}
		return nil
	})
	return out
}

func scanMacAppBundle(presetID string, appDir string) []Target {
	macosDir := filepath.Join(appDir, "Contents", "MacOS")
	entries, err := os.ReadDir(macosDir)
	if err != nil {
		return nil
	}
	var out []Target
	appTitle := strings.TrimSuffix(filepath.Base(appDir), filepath.Ext(appDir))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(macosDir, e.Name())
		info, err := e.Info()
		if err != nil || info.Mode()&0o111 == 0 {
			continue
		}
		t := buildTarget(presetID, appTitle, e.Name(), full)
		out = append(out, t)
	}
	return out
}

func targetFromFile(presetID string, fullPath string, goos string) (Target, bool) {
	base := filepath.Base(fullPath)
	lower := strings.ToLower(base)
	if goos == "windows" {
		if !strings.HasSuffix(lower, ".exe") {
			return Target{}, false
		}
		if strings.Contains(lower, "unins") || strings.Contains(lower, "setup") {
			return Target{}, false
		}
		return buildTarget(presetID, "", base, fullPath), true
	}

	// macOS / Linux: executable bit set, skip scripts/dylibs/so.
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return Target{}, false
	}
	if info.Mode()&0o111 == 0 {
		return Target{}, false
	}
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".dylib", ".so", ".sh", ".py", ".pl", ".rb", ".js", ".framework", ".plist", ".json", ".txt", ".png", ".icns":
		return Target{}, false
	}
	return buildTarget(presetID, "", base, fullPath), true
}

func buildTarget(presetID, fallbackName, execName, fullPath string) Target {
	cleanPath := filepath.Clean(fullPath)
	name := fallbackName
	kind := KindMain
	note := ""

	if presetID == "adobe" {
		cName, cKind, cNote := ClassifyAdobe(execName, cleanPath)
		name = cName
		kind = cKind
		note = cNote
	} else {
		if name == "" {
			name = strings.TrimSuffix(execName, filepath.Ext(execName))
		}
		kind = KindMain
	}

	return Target{
		ID:       targetID(cleanPath),
		PresetID: presetID,
		Name:     name,
		ExecName: execName,
		Path:     cleanPath,
		Kind:     kind,
		Note:     note,
	}
}

func targetID(path string) string {
	sum := sha1.Sum([]byte(strings.ToLower(filepath.Clean(path))))
	return hex.EncodeToString(sum[:8])
}

func addUnique(list *[]Target, seen map[string]struct{}, t Target) {
	key := strings.ToLower(filepath.Clean(t.Path))
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*list = append(*list, t)
}

func sortTargets(targets []Target) {
	rank := func(k TargetKind) int {
		switch k {
		case KindMain:
			return 0
		case KindBackground:
			return 1
		default:
			return 2
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		ri, rj := rank(targets[i].Kind), rank(targets[j].Kind)
		if ri != rj {
			return ri < rj
		}
		if targets[i].Name != targets[j].Name {
			return strings.ToLower(targets[i].Name) < strings.ToLower(targets[j].Name)
		}
		return strings.ToLower(targets[i].Path) < strings.ToLower(targets[j].Path)
	})
}

func pathDepth(p string) int {
	clean := filepath.Clean(p)
	if clean == "." || clean == string(filepath.Separator) {
		return 0
	}
	return strings.Count(clean, string(filepath.Separator))
}
