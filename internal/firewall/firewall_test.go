package firewall

import (
	"context"
	"strings"
	"testing"
)

func TestRuleNameDeterministic(t *testing.T) {
	a := RuleName(DirOut, `C:\Program Files\Adobe\Photoshop.exe`, "Adobe Photoshop")
	b := RuleName(DirOut, `c:\program files\adobe\photoshop.exe`, "Adobe Photoshop")
	if a != b {
		t.Fatalf("RuleName not case-insensitive deterministic: %q != %q", a, b)
	}
	if !strings.HasPrefix(a, RulePrefix+" [out] ") {
		t.Fatalf("unexpected prefix: %q", a)
	}
}

func TestMockEngineLifecycle(t *testing.T) {
	ctx := context.Background()
	eng := NewMockEngine("windows", "MockFW")

	specs := []ProgramSpec{
		{Path: `C:\Adobe\Photoshop.exe`, DisplayName: "Adobe Photoshop", PresetID: "adobe"},
		{Path: `C:\Adobe\AGSService.exe`, DisplayName: "AGSService", PresetID: "adobe"},
	}
	if err := eng.BlockPrograms(ctx, specs, DefaultApplyOptions()); err != nil {
		t.Fatalf("BlockPrograms failed: %v", err)
	}
	rules, err := eng.ListRules(ctx)
	if err != nil {
		t.Fatalf("ListRules failed: %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules (2 out + 2 in), got %d", len(rules))
	}

	if err := eng.UnblockPrograms(ctx, []string{`C:\Adobe\Photoshop.exe`}); err != nil {
		t.Fatalf("UnblockPrograms failed: %v", err)
	}
	rules, _ = eng.ListRules(ctx)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules remaining, got %d", len(rules))
	}

	removed, err := eng.RemoveAllManaged(ctx)
	if err != nil || removed != 2 {
		t.Fatalf("RemoveAllManaged = (%d, %v), want (2, nil)", removed, err)
	}
	rules, _ = eng.ListRules(ctx)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after RemoveAllManaged, got %d", len(rules))
	}

	if err := eng.FlushDNS(ctx); err != nil || eng.FlushCount() != 1 {
		t.Fatalf("FlushDNS failed: count=%d err=%v", eng.FlushCount(), err)
	}
}
