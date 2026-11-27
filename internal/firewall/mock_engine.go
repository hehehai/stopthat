package firewall

import (
	"context"
	"sort"
	"sync"
)

// MockEngine is an in-memory Engine implementation for tests and Linux previews.
type MockEngine struct {
	mu           sync.Mutex
	platform     string
	engineName   string
	rules        map[string]Rule // key: rule name
	diagnostics  []DiagnosticItem
	flushCount   int
	FailNextWith error
}

// NewMockEngine creates a new in-memory firewall engine.
func NewMockEngine(platform, engineName string) *MockEngine {
	return &MockEngine{
		platform:   platform,
		engineName: engineName,
		rules:      make(map[string]Rule),
		diagnostics: []DiagnosticItem{
			{
				Title:  "运行模式",
				Status: "开发模拟引擎",
				Detail: "当前在非 Windows/macOS 环境运行，规则仅保存在内存中用于预览和测试。",
				Level:  DiagInfo,
			},
			{
				Title:  "防火墙状态",
				Status: "正常",
				Detail: "模拟防火墙已就绪，支持出站与入站规则切换。",
				Level:  DiagOK,
			},
		},
	}
}

func (m *MockEngine) Capabilities() Capabilities {
	return Capabilities{
		Platform:         m.platform,
		EngineName:       m.engineName,
		SupportsOutbound: true,
		SupportsInbound:  true,
		Elevated:         true,
	}
}

func (m *MockEngine) ListRules(_ context.Context) ([]Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailNextWith != nil {
		err := m.FailNextWith
		m.FailNextWith = nil
		return nil, err
	}
	out := make([]Rule, 0, len(m.rules))
	for _, r := range m.rules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *MockEngine) BlockPrograms(_ context.Context, programs []ProgramSpec, opts ApplyOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailNextWith != nil {
		err := m.FailNextWith
		m.FailNextWith = nil
		return err
	}
	for _, p := range programs {
		prog := NormalizePath(p.Path)
		if prog == "" {
			continue
		}
		if opts.BlockOutbound {
			name := RuleName(DirOut, prog, p.DisplayName)
			m.rules[name] = Rule{
				Name:        name,
				ProgramPath: prog,
				Direction:   DirOut,
				DisplayName: p.DisplayName,
				Enabled:     true,
			}
		}
		if opts.BlockInbound {
			name := RuleName(DirIn, prog, p.DisplayName)
			m.rules[name] = Rule{
				Name:        name,
				ProgramPath: prog,
				Direction:   DirIn,
				DisplayName: p.DisplayName,
				Enabled:     true,
			}
		}
	}
	return nil
}

func (m *MockEngine) UnblockPrograms(_ context.Context, paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailNextWith != nil {
		err := m.FailNextWith
		m.FailNextWith = nil
		return err
	}
	wanted := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		wanted[PathKey(p)] = struct{}{}
	}
	for name, r := range m.rules {
		if _, ok := wanted[PathKey(r.ProgramPath)]; ok {
			delete(m.rules, name)
		}
	}
	return nil
}

func (m *MockEngine) RemoveAllManaged(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailNextWith != nil {
		err := m.FailNextWith
		m.FailNextWith = nil
		return 0, err
	}
	n := len(m.rules)
	clear(m.rules)
	return n, nil
}

func (m *MockEngine) Diagnostics(_ context.Context) []DiagnosticItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DiagnosticItem, len(m.diagnostics))
	copy(out, m.diagnostics)
	return out
}

func (m *MockEngine) FlushDNS(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailNextWith != nil {
		err := m.FailNextWith
		m.FailNextWith = nil
		return err
	}
	m.flushCount++
	return nil
}

// FlushCount returns how many times FlushDNS was called (for tests).
func (m *MockEngine) FlushCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushCount
}
