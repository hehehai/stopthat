package app

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/presets"
	"github.com/hehehai/stopthat/internal/store"
)

// Section IDs for sidebar navigation.
const (
	SectionAdobe       = "preset:adobe"
	SectionCustom      = "custom"
	SectionRules       = "rules"
	SectionDiagnostics = "diagnostics"
)

// Options customizes State construction (primarily for desktop wiring and tests).
type Options struct {
	ScanOS     string
	Invalidate func()
	PickFolder func(title string) (string, error)
	PickFile   func(title string) (string, error)
}

// State holds all application state and coordinates scanning + firewall actions.
type State struct {
	mu sync.Mutex

	engine firewall.Engine
	store  *store.Store
	opts   Options

	cfg           store.Config
	caps          firewall.Capabilities
	presets       []presets.Preset
	presetTargets map[string][]presets.Target
	customTargets []presets.Target
	rules         []firewall.Rule
	blocked       map[string]bool
	diagnostics   []firewall.DiagnosticItem

	// UI state (mutated on the UI thread)
	Section     string
	SearchQuery string
	KindFilter  int // 0=全部, 1=主程序, 2=后台组件, 3=已阻止, 4=未阻止
	PathInput   string

	StatusMsg string
	StatusErr bool
	Busy      bool
}

// NewState initializes State, loads persisted settings, and performs an initial scan.
func NewState(engine firewall.Engine, st *store.Store, opts Options) *State {
	if opts.ScanOS == "" {
		opts.ScanOS = runtime.GOOS
	}
	cfg := store.DefaultConfig()
	if st != nil {
		if loaded, err := st.Load(); err == nil {
			cfg = loaded
		}
	}
	s := &State{
		engine:        engine,
		store:         st,
		opts:          opts,
		cfg:           cfg,
		caps:          engine.Capabilities(),
		presets:       presets.Builtin(),
		presetTargets: make(map[string][]presets.Target),
		blocked:       make(map[string]bool),
		Section:       SectionAdobe,
	}
	s.RefreshAll()
	return s
}

// SetInvalidate registers a callback that triggers a window redraw.
func (s *State) SetInvalidate(fn func()) {
	s.mu.Lock()
	s.opts.Invalidate = fn
	s.mu.Unlock()
}

func (s *State) notifyUI() {
	s.mu.Lock()
	fn := s.opts.Invalidate
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// RefreshAll rescans preset suites, custom paths, active rules, and diagnostics.
func (s *State) RefreshAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.caps = s.engine.Capabilities()
	for _, p := range s.presets {
		extra := s.cfg.ExtraPresetRoots[p.ID]
		s.presetTargets[p.ID] = presets.ScanPreset(p, presets.ScanOptions{
			ExtraRoots: extra,
			OS:         s.opts.ScanOS,
		})
	}
	s.rescanCustomLocked()
	s.refreshRulesLocked(ctx)
	s.diagnostics = s.engine.Diagnostics(ctx)
}

func (s *State) rescanCustomLocked() {
	seen := make(map[string]struct{})
	var all []presets.Target
	for _, p := range s.cfg.CustomPaths {
		for _, t := range presets.ScanCustomPath(p, s.opts.ScanOS) {
			k := firewall.PathKey(t.Path)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			all = append(all, t)
		}
	}
	s.customTargets = all
}

func (s *State) refreshRulesLocked(ctx context.Context) {
	rules, err := s.engine.ListRules(ctx)
	if err != nil {
		s.setStatusLocked("读取防火墙规则失败："+err.Error(), true)
		return
	}
	s.rules = rules
	blocked := make(map[string]bool, len(rules))
	for _, r := range rules {
		if r.Enabled {
			blocked[firewall.PathKey(r.ProgramPath)] = true
		}
	}
	s.blocked = blocked
}

func (s *State) saveConfigLocked() {
	if s.store != nil {
		_ = s.store.Save(s.cfg)
	}
}

func (s *State) setStatusLocked(msg string, isErr bool) {
	s.StatusMsg = msg
	s.StatusErr = isErr
}

// ApplyOptions returns the current outbound/inbound blocking preference.
func (s *State) ApplyOptions() firewall.ApplyOptions {
	return firewall.ApplyOptions{
		BlockOutbound: s.cfg.BlockOutbound,
		BlockInbound:  s.cfg.BlockInbound,
	}
}

// IsBlocked reports whether a given executable path currently has an active block rule.
func (s *State) IsBlocked(path string) bool {
	return s.blocked[firewall.PathKey(path)]
}

// PresetByID returns a built-in preset by ID.
func (s *State) PresetByID(id string) (presets.Preset, bool) {
	for _, p := range s.presets {
		if p.ID == id {
			return p, true
		}
	}
	return presets.Preset{}, false
}

// PresetStats returns (total, mainCount, bgCount, blockedCount) for a preset.
func (s *State) PresetStats(presetID string) (total, mainCount, bgCount, blockedCount int) {
	targets := s.presetTargets[presetID]
	total = len(targets)
	for _, t := range targets {
		switch t.Kind {
		case presets.KindMain:
			mainCount++
		case presets.KindBackground:
			bgCount++
		}
		if s.IsBlocked(t.Path) {
			blockedCount++
		}
	}
	return
}

// FilteredTargets returns targets matching the current search query and filter tab.
func (s *State) FilteredTargets(targets []presets.Target) []presets.Target {
	q := strings.ToLower(strings.TrimSpace(s.SearchQuery))
	out := make([]presets.Target, 0, len(targets))
	for _, t := range targets {
		blocked := s.IsBlocked(t.Path)
		switch s.KindFilter {
		case 1:
			if t.Kind != presets.KindMain {
				continue
			}
		case 2:
			if t.Kind != presets.KindBackground {
				continue
			}
		case 3:
			if !blocked {
				continue
			}
		case 4:
			if blocked {
				continue
			}
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(t.Name), q) &&
				!strings.Contains(strings.ToLower(t.ExecName), q) &&
				!strings.Contains(strings.ToLower(t.Path), q) {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// ToggleTarget blocks or unblocks a single executable.
func (s *State) ToggleTarget(t presets.Target, block bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	if block {
		opts := s.ApplyOptions()
		if !opts.BlockOutbound && !opts.BlockInbound {
			s.setStatusLocked("请至少勾选「阻止出站」或「阻止入站」中的一项", true)
			return
		}
		err := s.engine.BlockPrograms(ctx, []firewall.ProgramSpec{{
			Path:        t.Path,
			DisplayName: t.Name,
			PresetID:    t.PresetID,
		}}, opts)
		if err != nil {
			s.setStatusLocked(err.Error(), true)
			return
		}
		s.refreshRulesLocked(ctx)
		s.setStatusLocked(fmt.Sprintf("已阻止「%s」联网", t.Name), false)
		return
	}

	if err := s.engine.UnblockPrograms(ctx, []string{t.Path}); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked(fmt.Sprintf("已恢复「%s」联网", t.Name), false)
}

// BlockPresetAll blocks all discovered programs in a preset (honoring IncludeBackground).
func (s *State) BlockPresetAll(presetID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	targets := s.presetTargets[presetID]
	if len(targets) == 0 {
		s.setStatusLocked("未扫描到可管理的程序，请先添加自定义安装目录后再扫描", true)
		return
	}
	opts := s.ApplyOptions()
	if !opts.BlockOutbound && !opts.BlockInbound {
		s.setStatusLocked("请至少勾选「阻止出站」或「阻止入站」中的一项", true)
		return
	}

	var specs []firewall.ProgramSpec
	for _, t := range targets {
		if !s.cfg.IncludeBackground && t.Kind == presets.KindBackground {
			continue
		}
		specs = append(specs, firewall.ProgramSpec{
			Path:        t.Path,
			DisplayName: t.Name,
			PresetID:    t.PresetID,
		})
	}
	if len(specs) == 0 {
		s.setStatusLocked("当前筛选条件下没有可阻止的程序", true)
		return
	}
	if err := s.engine.BlockPrograms(ctx, specs, opts); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked(fmt.Sprintf("已为 %d 个程序添加联网阻止规则", len(specs)), false)
}

// UnblockPresetAll removes block rules for all targets in a preset.
func (s *State) UnblockPresetAll(presetID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	targets := s.presetTargets[presetID]
	var paths []string
	for _, t := range targets {
		if s.IsBlocked(t.Path) {
			paths = append(paths, t.Path)
		}
	}
	if len(paths) == 0 {
		s.setStatusLocked("当前套件下没有已阻止的程序", false)
		return
	}
	if err := s.engine.UnblockPrograms(ctx, paths); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked(fmt.Sprintf("已恢复 %d 个程序的联网权限", len(paths)), false)
}

// AddExtraPresetRoot adds a custom installation directory to a preset and rescans.
func (s *State) AddExtraPresetRoot(presetID, dir string) {
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if dir == "" {
		return
	}
	s.mu.Lock()
	if s.cfg.ExtraPresetRoots == nil {
		s.cfg.ExtraPresetRoots = make(map[string][]string)
	}
	s.cfg.ExtraPresetRoots[presetID] = store.AddUniquePath(s.cfg.ExtraPresetRoots[presetID], dir)
	s.saveConfigLocked()
	s.mu.Unlock()

	s.RefreshAll()

	s.mu.Lock()
	s.setStatusLocked(fmt.Sprintf("已添加目录并重新扫描：%s", dir), false)
	s.mu.Unlock()
}

// AddCustomPath adds a custom file or folder to the Custom section and rescans.
func (s *State) AddCustomPath(p string) {
	p = strings.Trim(strings.TrimSpace(p), `"'`)
	if p == "" {
		return
	}
	s.mu.Lock()
	s.cfg.CustomPaths = store.AddUniquePath(s.cfg.CustomPaths, p)
	s.saveConfigLocked()
	s.rescanCustomLocked()
	count := len(s.customTargets)
	s.setStatusLocked(fmt.Sprintf("已添加自定义路径（当前共扫描到 %d 个可执行文件）", count), false)
	s.mu.Unlock()
}

// RemoveCustomPath removes a path from the Custom section.
func (s *State) RemoveCustomPath(p string) {
	s.mu.Lock()
	s.cfg.CustomPaths = store.RemovePath(s.cfg.CustomPaths, p)
	s.saveConfigLocked()
	s.rescanCustomLocked()
	s.setStatusLocked("已移除自定义路径", false)
	s.mu.Unlock()
}

// BlockCustomAll blocks all discovered custom targets.
func (s *State) BlockCustomAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.customTargets) == 0 {
		s.setStatusLocked("自定义列表中暂无可执行文件", true)
		return
	}
	specs := make([]firewall.ProgramSpec, 0, len(s.customTargets))
	for _, t := range s.customTargets {
		specs = append(specs, firewall.ProgramSpec{
			Path:        t.Path,
			DisplayName: t.Name,
			PresetID:    "custom",
		})
	}
	if err := s.engine.BlockPrograms(ctx, specs, s.ApplyOptions()); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked(fmt.Sprintf("已阻止 %d 个自定义程序联网", len(specs)), false)
}

// UnblockPath removes all StopThat rules for a single program path.
func (s *State) UnblockPath(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.engine.UnblockPrograms(ctx, []string{path}); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked("已移除该程序的拦截规则", false)
}

// RemoveAllManagedRules removes every firewall rule created by StopThat.
func (s *State) RemoveAllManagedRules() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := s.engine.RemoveAllManaged(ctx)
	if err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.refreshRulesLocked(ctx)
	s.setStatusLocked(fmt.Sprintf("已一键还原，共清理 %d 条 StopThat 规则", n), false)
}

// FlushDNS triggers OS DNS cache flush.
func (s *State) FlushDNS() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.engine.FlushDNS(ctx); err != nil {
		s.setStatusLocked(err.Error(), true)
		return
	}
	s.setStatusLocked("DNS 缓存已刷新", false)
}

// RefreshDiagnostics reruns environment checks.
func (s *State) RefreshDiagnostics() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.caps = s.engine.Capabilities()
	s.diagnostics = s.engine.Diagnostics(ctx)
	s.setStatusLocked("已完成网络环境体检", false)
}

// SavePreferences persists current checkbox preferences.
func (s *State) SavePreferences() {
	s.mu.Lock()
	s.saveConfigLocked()
	s.mu.Unlock()
}

// PickAndAddPresetFolder opens a native folder picker asynchronously and adds it to presetID.
func (s *State) PickAndAddPresetFolder(presetID string) {
	picker := s.opts.PickFolder
	if picker == nil {
		return
	}
	go func() {
		dir, err := picker("选择软件安装目录（如 D:\\Adobe）")
		if err != nil || strings.TrimSpace(dir) == "" {
			return
		}
		s.AddExtraPresetRoot(presetID, dir)
		s.notifyUI()
	}()
}

// PickAndAddCustomPath opens a native picker for a folder or executable.
func (s *State) PickAndAddCustomPath(directory bool) {
	var picker func(string) (string, error)
	title := "选择要管理联网的程序"
	if directory {
		picker = s.opts.PickFolder
		title = "选择要管理联网的软件目录"
	} else {
		picker = s.opts.PickFile
	}
	if picker == nil {
		return
	}
	go func() {
		p, err := picker(title)
		if err != nil || strings.TrimSpace(p) == "" {
			return
		}
		s.AddCustomPath(p)
		s.notifyUI()
	}()
}
