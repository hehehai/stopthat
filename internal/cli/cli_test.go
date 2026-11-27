package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hehehai/stopthat/internal/firewall"
)

func TestShouldHandle(t *testing.T) {
	if ShouldHandle(nil) {
		t.Fatal("empty args should launch GUI")
	}
	if ShouldHandle([]string{"-psn_0_12345"}) {
		t.Fatal("macOS Finder -psn arg should launch GUI")
	}
	if !ShouldHandle([]string{"scan", "--json"}) {
		t.Fatal("scan should be handled by CLI")
	}
}

func TestCLIScanBlockRulesRestore(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "sample-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	eng := firewall.NewMockEngine("linux", "MockFW")
	var out, errBuf bytes.Buffer

	if code := Run([]string{"block", "--path", bin, "--json"}, eng, nil, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("block failed: code=%d stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), `"blocked_count": 1`) {
		t.Fatalf("unexpected block output: %s", out.String())
	}

	out.Reset()
	if code := Run([]string{"rules", "--json"}, eng, nil, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("rules failed: code=%d", code)
	}
	if !strings.Contains(out.String(), `"count": 2`) {
		t.Fatalf("expected 2 rules (out+in), got: %s", out.String())
	}

	out.Reset()
	if code := Run([]string{"restore", "--json"}, eng, nil, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("restore failed: code=%d", code)
	}
	if !strings.Contains(out.String(), `"removed_rules": 2`) {
		t.Fatalf("unexpected restore output: %s", out.String())
	}

	out.Reset()
	if code := Run([]string{"doctor", "--json"}, eng, nil, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("doctor failed: code=%d", code)
	}
	if !strings.Contains(out.String(), `"diagnostics"`) {
		t.Fatalf("unexpected doctor output: %s", out.String())
	}
}
