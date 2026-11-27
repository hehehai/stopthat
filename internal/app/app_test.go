package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/store"
)

func TestStopThatEndToEndUI(t *testing.T) {
	// Prepare a fake Adobe directory with main + background executables.
	dir := t.TempDir()
	for _, rel := range []string{
		"Adobe Photoshop 2025/Photoshop.exe",
		"AdobeGCClient/AGSService.exe",
	} {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	eng := firewall.NewMockEngine("windows", "Mock Windows Firewall")
	st := store.New(filepath.Join(t.TempDir(), "config.json"))
	state := NewState(eng, st, Options{ScanOS: "windows"})
	state.AddExtraPresetRoot("adobe", dir)

	total, mainCount, bgCount, blocked := state.PresetStats("adobe")
	if total != 2 || mainCount != 1 || bgCount != 1 || blocked != 0 {
		t.Fatalf("unexpected initial stats: total=%d main=%d bg=%d blocked=%d", total, mainCount, bgCount, blocked)
	}

	tester := ui.NewTester(state.View, 980, 660)
	tester.Frame()

	// Block all Adobe targets.
	state.BlockPresetAll("adobe")
	tester.Frame()

	_, _, _, blocked = state.PresetStats("adobe")
	if blocked != 2 {
		t.Fatalf("expected 2 blocked targets, got %d", blocked)
	}
	if len(state.rules) != 4 {
		t.Fatalf("expected 4 rules (out+in for 2 targets), got %d", len(state.rules))
	}

	// Render Rules view and remove all managed rules.
	state.Section = SectionRules
	tester.Frame()
	state.RemoveAllManagedRules()
	tester.Frame()
	if len(state.rules) != 0 {
		t.Fatalf("expected 0 rules after RemoveAllManagedRules, got %d", len(state.rules))
	}

	// Render Custom view & Diagnostics view to verify zero layout/build panics.
	state.Section = SectionCustom
	state.AddCustomPath(filepath.Join(dir, "Adobe Photoshop 2025", "Photoshop.exe"))
	tester.Frame()
	if len(state.customTargets) != 1 {
		t.Fatalf("expected 1 custom target, got %d", len(state.customTargets))
	}

	state.Section = SectionDiagnostics
	state.FlushDNS()
	tester.Frame()
	if eng.FlushCount() != 1 {
		t.Fatalf("expected FlushDNS count=1, got %d", eng.FlushCount())
	}
}
