package presets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanPresetWindowsAdobe(t *testing.T) {
	tmp := t.TempDir()
	files := []string{
		filepath.Join(tmp, "Adobe Photoshop 2025", "Photoshop.exe"),
		filepath.Join(tmp, "Adobe Illustrator 2025", "Support Files", "Contents", "Windows", "Illustrator.exe"),
		filepath.Join(tmp, "AdobeGCClient", "AdobeGCClient.exe"),
		filepath.Join(tmp, "AdobeGCClient", "AGSService.exe"),
		filepath.Join(tmp, "AdobeGCClient", "AGMService.exe"),
		filepath.Join(tmp, "AdobeGCClient", "agcinvokerutility.exe"),
		filepath.Join(tmp, "Adobe Photoshop 2025", "unins000.exe"), // should be skipped
		filepath.Join(tmp, "Adobe Photoshop 2025", "readme.txt"),   // should be skipped
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	p := Preset{
		ID:   "adobe",
		Name: "Adobe Creative Cloud",
	}
	got := ScanPreset(p, ScanOptions{
		ExtraRoots: []string{tmp},
		OS:         "windows",
	})
	if len(got) != 6 {
		t.Fatalf("expected 6 executables, got %d: %+v", len(got), got)
	}

	kinds := make(map[string]TargetKind)
	for _, item := range got {
		kinds[item.ExecName] = item.Kind
	}
	if kinds["Photoshop.exe"] != KindMain {
		t.Errorf("Photoshop.exe kind = %v, want %v", kinds["Photoshop.exe"], KindMain)
	}
	if kinds["Illustrator.exe"] != KindMain {
		t.Errorf("Illustrator.exe kind = %v, want %v", kinds["Illustrator.exe"], KindMain)
	}
	for _, bg := range []string{"AdobeGCClient.exe", "AGSService.exe", "AGMService.exe", "agcinvokerutility.exe"} {
		if kinds[bg] != KindBackground {
			t.Errorf("%s kind = %v, want %v", bg, kinds[bg], KindBackground)
		}
	}
}

func TestScanMacAppBundle(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "Adobe Photoshop 2025.app", "Contents", "MacOS", "Adobe Photoshop 2025")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := Preset{ID: "adobe", Name: "Adobe Creative Cloud"}
	got := ScanPreset(p, ScanOptions{ExtraRoots: []string{tmp}, OS: "darwin"})
	if len(got) != 1 {
		t.Fatalf("expected 1 macOS executable, got %d: %+v", len(got), got)
	}
	if got[0].Kind != KindMain || got[0].Name != "Adobe Photoshop" {
		t.Errorf("unexpected target: %+v", got[0])
	}
}

func TestScanCustomPathStripsQuotes(t *testing.T) {
	tmp := t.TempDir()
	exe := filepath.Join(tmp, "MyApp.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ScanCustomPath(`"`+exe+`"`, "windows")
	if len(got) != 1 || got[0].ExecName != "MyApp.exe" {
		t.Fatalf("expected 1 target from quoted path, got %+v", got)
	}
}
