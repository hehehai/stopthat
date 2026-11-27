package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds persisted user settings and custom scan paths.
type Config struct {
	ExtraPresetRoots  map[string][]string `json:"extra_preset_roots,omitempty"`
	CustomPaths       []string            `json:"custom_paths,omitempty"`
	BlockOutbound     bool                `json:"block_outbound"`
	BlockInbound      bool                `json:"block_inbound"`
	IncludeBackground bool                `json:"include_background"`
}

// DefaultConfig returns sensible defaults matching recommended firewall settings.
func DefaultConfig() Config {
	return Config{
		ExtraPresetRoots:  make(map[string][]string),
		BlockOutbound:     true,
		BlockInbound:      true,
		IncludeBackground: true,
	}
}

// Store reads and writes Config JSON to disk.
type Store struct {
	path string
}

// NewDefault opens the standard per-user config store for StopThat.
func NewDefault() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return New(filepath.Join(dir, "StopThat", "config.json")), nil
}

// New opens a store backed by the given file path.
func New(path string) *Store {
	return &Store{path: path}
}

// Load reads Config from disk, returning DefaultConfig if it does not exist yet.
func (s *Store) Load() (Config, error) {
	cfg := DefaultConfig()
	if s == nil || s.path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	if cfg.ExtraPresetRoots == nil {
		cfg.ExtraPresetRoots = make(map[string][]string)
	}
	return cfg, nil
}

// Save writes Config atomically to disk.
func (s *Store) Save(cfg Config) error {
	if s == nil || s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// AddUniquePath appends p to list if not already present (case-insensitive).
func AddUniquePath(list []string, p string) []string {
	p = strings.Trim(strings.TrimSpace(p), `"'`)
	if p == "" {
		return list
	}
	clean := filepath.Clean(p)
	key := strings.ToLower(clean)
	for _, existing := range list {
		if strings.ToLower(filepath.Clean(existing)) == key {
			return list
		}
	}
	return append(list, clean)
}

// RemovePath removes p from list (case-insensitive).
func RemovePath(list []string, p string) []string {
	key := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	out := make([]string, 0, len(list))
	for _, existing := range list {
		if strings.ToLower(filepath.Clean(existing)) == key {
			continue
		}
		out = append(out, existing)
	}
	return out
}
