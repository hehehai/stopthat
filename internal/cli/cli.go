package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/mcp"
	"github.com/hehehai/stopthat/internal/presets"
	"github.com/hehehai/stopthat/internal/store"
)

// Version is the current StopThat CLI/App version.
const Version = "0.1.0"

// ShouldHandle reports whether argv (os.Args[1:]) should be handled by the CLI/MCP
// dispatcher instead of launching the desktop GUI.
func ShouldHandle(args []string) bool {
	if len(args) == 0 {
		return false
	}
	// macOS Finder passes -psn_0_XXXXXX when launching a .app bundle.
	if strings.HasPrefix(args[0], "-psn_") {
		return false
	}
	return true
}

// Run executes a StopThat CLI or MCP command and returns the process exit code.
func Run(args []string, engine firewall.Engine, st *store.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}
	cmd := strings.ToLower(strings.TrimSpace(args[0]))
	rest := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "StopThat %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return 0
	case "mcp":
		srv := mcp.NewServer(engine, st, runtime.GOOS)
		if err := srv.ServeStdio(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "stopthat mcp: %v\n", err)
			return 1
		}
		return 0
	case "scan":
		return runScan(rest, engine, st, stdout, stderr)
	case "block":
		return runBlock(rest, engine, st, stdout, stderr)
	case "unblock":
		return runUnblock(rest, engine, st, stdout, stderr)
	case "rules":
		return runRules(rest, engine, stdout, stderr)
	case "restore":
		return runRestore(rest, engine, stdout, stderr)
	case "doctor":
		return runDoctor(rest, engine, stdout, stderr)
	case "flush-dns":
		return runFlushDNS(rest, engine, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "未知命令: %s\n\n", cmd)
		printUsage(stderr)
		return 2
	}
}

func runScan(args []string, engine firewall.Engine, st *store.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	presetID := fs.String("preset", "adobe", "预设套件（目前支持 adobe）")
	extraDir := fs.String("dir", "", "额外扫描目录")
	customPath := fs.String("path", "", "扫描指定程序或目录（不使用预设）")
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var targets []presets.Target
	if strings.TrimSpace(*customPath) != "" {
		targets = presets.ScanCustomPath(*customPath, runtime.GOOS)
	} else {
		var extra []string
		if st != nil {
			if cfg, err := st.Load(); err == nil {
				extra = append(extra, cfg.ExtraPresetRoots[*presetID]...)
			}
		}
		if strings.TrimSpace(*extraDir) != "" {
			extra = append(extra, *extraDir)
		}
		targets = presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{ExtraRoots: extra})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rules, _ := engine.ListRules(ctx)
	blocked := make(map[string]bool, len(rules))
	for _, r := range rules {
		if r.Enabled {
			blocked[firewall.PathKey(r.ProgramPath)] = true
		}
	}

	if *asJSON {
		type item struct {
			presets.Target
			Blocked bool `json:"blocked"`
		}
		out := make([]item, 0, len(targets))
		for _, t := range targets {
			out = append(out, item{Target: t, Blocked: blocked[firewall.PathKey(t.Path)]})
		}
		writeJSON(stdout, map[string]any{"count": len(out), "targets": out})
		return 0
	}

	fmt.Fprintf(stdout, "共扫描到 %d 个程序：\n", len(targets))
	for _, t := range targets {
		state := "允许联网"
		if blocked[firewall.PathKey(t.Path)] {
			state = "已阻止"
		}
		fmt.Fprintf(stdout, "  [%s] [%s] %s — %s\n", state, t.Kind.Label(), t.Name, t.Path)
	}
	return 0
}

func runBlock(args []string, engine firewall.Engine, st *store.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("block", flag.ContinueOnError)
	fs.SetOutput(stderr)
	presetID := fs.String("preset", "adobe", "预设套件（目前支持 adobe）")
	extraDir := fs.String("dir", "", "额外扫描目录")
	customPath := fs.String("path", "", "阻止指定程序或目录（不使用预设）")
	noBG := fs.Bool("no-bg", false, "不包含后台关联组件")
	outOnly := fs.Bool("out-only", false, "仅阻止出站连接")
	inOnly := fs.Bool("in-only", false, "仅阻止入站连接")
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := firewall.DefaultApplyOptions()
	if *outOnly {
		opts.BlockOutbound = true
		opts.BlockInbound = false
	} else if *inOnly {
		opts.BlockOutbound = false
		opts.BlockInbound = true
	}

	var specs []firewall.ProgramSpec
	if strings.TrimSpace(*customPath) != "" {
		for _, t := range presets.ScanCustomPath(*customPath, runtime.GOOS) {
			specs = append(specs, firewall.ProgramSpec{Path: t.Path, DisplayName: t.Name, PresetID: "custom"})
		}
	} else {
		var extra []string
		if st != nil {
			if cfg, err := st.Load(); err == nil {
				extra = append(extra, cfg.ExtraPresetRoots[*presetID]...)
			}
		}
		if strings.TrimSpace(*extraDir) != "" {
			extra = append(extra, *extraDir)
		}
		for _, t := range presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{ExtraRoots: extra}) {
			if *noBG && t.Kind == presets.KindBackground {
				continue
			}
			specs = append(specs, firewall.ProgramSpec{Path: t.Path, DisplayName: t.Name, PresetID: t.PresetID})
		}
	}

	if len(specs) == 0 {
		msg := "未找到可阻止的程序，请使用 --dir 或 --path 指定安装目录"
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": msg})
		} else {
			fmt.Fprintln(stderr, msg)
		}
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := engine.BlockPrograms(ctx, specs, opts); err != nil {
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(stderr, "阻止失败: %v\n", err)
		}
		return 1
	}

	if *asJSON {
		writeJSON(stdout, map[string]any{"ok": true, "blocked_count": len(specs), "options": opts})
	} else {
		fmt.Fprintf(stdout, "已为 %d 个程序添加防火墙阻止规则。\n", len(specs))
	}
	return 0
}

func runUnblock(args []string, engine firewall.Engine, st *store.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("unblock", flag.ContinueOnError)
	fs.SetOutput(stderr)
	presetID := fs.String("preset", "adobe", "预设套件（目前支持 adobe）")
	extraDir := fs.String("dir", "", "额外扫描目录")
	customPath := fs.String("path", "", "恢复指定程序或目录")
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var paths []string
	if strings.TrimSpace(*customPath) != "" {
		for _, t := range presets.ScanCustomPath(*customPath, runtime.GOOS) {
			paths = append(paths, t.Path)
		}
		if len(paths) == 0 {
			paths = append(paths, *customPath)
		}
	} else {
		var extra []string
		if st != nil {
			if cfg, err := st.Load(); err == nil {
				extra = append(extra, cfg.ExtraPresetRoots[*presetID]...)
			}
		}
		if strings.TrimSpace(*extraDir) != "" {
			extra = append(extra, *extraDir)
		}
		for _, t := range presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{ExtraRoots: extra}) {
			paths = append(paths, t.Path)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := engine.UnblockPrograms(ctx, paths); err != nil {
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(stderr, "恢复失败: %v\n", err)
		}
		return 1
	}
	if *asJSON {
		writeJSON(stdout, map[string]any{"ok": true, "unblocked_paths": len(paths)})
	} else {
		fmt.Fprintf(stdout, "已恢复 %d 个路径的联网权限。\n", len(paths))
	}
	return 0
}

func runRules(args []string, engine firewall.Engine, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rules, err := engine.ListRules(ctx)
	if err != nil {
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(stderr, "读取规则失败: %v\n", err)
		}
		return 1
	}
	if *asJSON {
		writeJSON(stdout, map[string]any{"count": len(rules), "rules": rules})
		return 0
	}
	fmt.Fprintf(stdout, "当前共 %d 条 StopThat 防火墙规则：\n", len(rules))
	for _, r := range rules {
		fmt.Fprintf(stdout, "  [%s] %s — %s\n", r.Direction, r.DisplayName, r.ProgramPath)
	}
	return 0
}

func runRestore(args []string, engine firewall.Engine, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := engine.RemoveAllManaged(ctx)
	if err != nil {
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(stderr, "一键还原失败: %v\n", err)
		}
		return 1
	}
	if *asJSON {
		writeJSON(stdout, map[string]any{"ok": true, "removed_rules": n})
	} else {
		fmt.Fprintf(stdout, "已一键还原，共清理 %d 条 StopThat 规则。\n", n)
	}
	return 0
}

func runDoctor(args []string, engine firewall.Engine, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	caps := engine.Capabilities()
	diags := engine.Diagnostics(ctx)
	if *asJSON {
		writeJSON(stdout, map[string]any{"capabilities": caps, "diagnostics": diags})
		return 0
	}
	fmt.Fprintf(stdout, "防火墙引擎：%s (%s)\n", caps.EngineName, caps.Platform)
	for _, d := range diags {
		fmt.Fprintf(stdout, "  [%s] %s: %s\n      %s\n", strings.ToUpper(string(d.Level)), d.Title, d.Status, d.Detail)
	}
	return 0
}

func runFlushDNS(args []string, engine firewall.Engine, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("flush-dns", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := engine.FlushDNS(ctx); err != nil {
		if *asJSON {
			writeJSON(stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(stderr, "刷新 DNS 失败: %v\n", err)
		}
		return 1
	}
	if *asJSON {
		writeJSON(stdout, map[string]any{"ok": true, "message": "DNS 缓存已刷新"})
	} else {
		fmt.Fprintln(stdout, "DNS 缓存已刷新。")
	}
	return 0
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func printUsage(w io.Writer) {
	exe := "stopthat"
	if len(os.Args) > 0 && strings.TrimSpace(os.Args[0]) != "" {
		exe = os.Args[0]
	}
	fmt.Fprintf(w, `StopThat %s — 跨平台应用联网管理工具（GUI / CLI / MCP）

直接无参数运行可打开图形界面；传入子命令可用于终端或 AI Agent 自动化操作：

用法:
  %s <命令> [选项]

命令:
  scan       扫描预设套件（默认 adobe）或指定路径的可执行文件
  block      添加防火墙阻止规则（默认同时阻止出站与入站）
  unblock    恢复指定程序或预设套件的联网权限
  rules      列出所有由 StopThat 管理的生效规则
  restore    一键清理所有 StopThat 规则，彻底还原网络设置
  doctor     执行网络与防火墙环境体检（权限/防火墙/第三方安全软件/代理）
  flush-dns  刷新系统 DNS 缓存
  mcp        以 stdio 模式启动 MCP (Model Context Protocol) 服务
  version    打印版本号

所有查询/操作命令均支持 --json 输出，方便 AI Agent 解析。
`, Version, exe)
}
