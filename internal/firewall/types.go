package firewall

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// RulePrefix namespaces all firewall rules created by StopThat so we never
// modify or delete user or OS rules created outside this app.
const RulePrefix = "StopThat"

// Direction specifies traffic direction for a firewall rule.
type Direction string

const (
	DirOut Direction = "out"
	DirIn  Direction = "in"
)

// ApplyOptions controls which directions to block when adding a rule.
type ApplyOptions struct {
	BlockOutbound bool
	BlockInbound  bool
}

// DefaultApplyOptions returns the recommended blocking options (both outbound and inbound).
func DefaultApplyOptions() ApplyOptions {
	return ApplyOptions{
		BlockOutbound: true,
		BlockInbound:  true,
	}
}

// ProgramSpec identifies an executable to block.
type ProgramSpec struct {
	Path        string
	DisplayName string
	PresetID    string
}

// Rule represents an active firewall rule managed by StopThat.
type Rule struct {
	Name        string    `json:"name"`
	ProgramPath string    `json:"program_path"`
	Direction   Direction `json:"direction"`
	DisplayName string    `json:"display_name"`
	Enabled     bool      `json:"enabled"`
}

// DiagLevel indicates the severity of a network diagnostic check.
type DiagLevel string

const (
	DiagOK    DiagLevel = "ok"
	DiagWarn  DiagLevel = "warn"
	DiagError DiagLevel = "error"
	DiagInfo  DiagLevel = "info"
)

// DiagnosticItem is a single finding from the environment diagnostics check.
type DiagnosticItem struct {
	Title  string    `json:"title"`
	Status string    `json:"status"`
	Detail string    `json:"detail"`
	Level  DiagLevel `json:"level"`
}

// Capabilities describes what the current platform firewall backend supports.
type Capabilities struct {
	Platform           string
	EngineName         string
	SupportsOutbound   bool
	SupportsInbound    bool
	Elevated           bool
	NeedsElevationHint string
	OutboundNote       string
}

// Engine abstracts OS-specific application firewall operations.
type Engine interface {
	Capabilities() Capabilities
	ListRules(ctx context.Context) ([]Rule, error)
	BlockPrograms(ctx context.Context, programs []ProgramSpec, opts ApplyOptions) error
	UnblockPrograms(ctx context.Context, paths []string) error
	RemoveAllManaged(ctx context.Context) (int, error)
	Diagnostics(ctx context.Context) []DiagnosticItem
	FlushDNS(ctx context.Context) error
}

// RuleName builds a deterministic, collision-free rule name for a program & direction.
func RuleName(dir Direction, programPath, displayName string) string {
	clean := NormalizePath(programPath)
	sum := sha1.Sum([]byte(strings.ToLower(clean)))
	shortHash := hex.EncodeToString(sum[:4])
	safeName := sanitizeName(displayName)
	if safeName == "" {
		safeName = sanitizeName(filepath.Base(clean))
	}
	return RulePrefix + " [" + string(dir) + "] " + shortHash + " " + safeName
}

// NormalizePath cleans a path for consistent comparison.
func NormalizePath(p string) string {
	return filepath.Clean(strings.TrimSpace(p))
}

// PathKey returns a case-insensitive lookup key for comparing executable paths.
func PathKey(p string) string {
	return strings.ToLower(NormalizePath(p))
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(`"`, "", `'`, "", "\r", "", "\n", "", "|", "-", ";", "-")
	s = replacer.Replace(s)
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}
