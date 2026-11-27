package store

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "StopThat", "config.json")
	st := New(p)

	cfg, err := st.Load()
	if err != nil {
		t.Fatalf("initial Load failed: %v", err)
	}
	if !cfg.BlockOutbound || !cfg.BlockInbound || !cfg.IncludeBackground {
		t.Fatalf("unexpected default config: %+v", cfg)
	}

	cfg.CustomPaths = AddUniquePath(cfg.CustomPaths, `"/tmp/App"`)
	cfg.CustomPaths = AddUniquePath(cfg.CustomPaths, `/tmp/App`) // duplicate
	if len(cfg.CustomPaths) != 1 {
		t.Fatalf("expected 1 unique custom path, got %v", cfg.CustomPaths)
	}
	cfg.BlockInbound = false
	if err := st.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := st.Load()
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if loaded.BlockInbound || len(loaded.CustomPaths) != 1 {
		t.Fatalf("loaded mismatch: %+v", loaded)
	}

	loaded.CustomPaths = RemovePath(loaded.CustomPaths, `/tmp/App`)
	if len(loaded.CustomPaths) != 0 {
		t.Fatalf("expected empty after remove, got %v", loaded.CustomPaths)
	}
}
