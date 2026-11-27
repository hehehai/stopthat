package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hehehai/stopthat/internal/firewall"
)

func TestMCPServerInitializeListAndCall(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Photoshop.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := firewall.NewMockEngine("windows", "MockFW")
	srv := NewServer(eng, nil, "windows")

	reqs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"stopthat_block","arguments":{"paths":["` + filepath.ToSlash(exe) + `"]}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"stopthat_list_rules","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"stopthat_restore_all","arguments":{}}}`,
	}

	var out bytes.Buffer
	if err := srv.ServeStdio(strings.NewReader(strings.Join(reqs, "\n")+"\n"), &out); err != nil {
		t.Fatalf("ServeStdio failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses, got %d:\n%s", len(lines), out.String())
	}

	for i, line := range lines {
		var resp rpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if resp.Error != nil {
			t.Fatalf("line %d unexpected error: %+v", i, resp.Error)
		}
	}
}
