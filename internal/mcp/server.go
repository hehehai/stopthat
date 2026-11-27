package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/presets"
	"github.com/hehehai/stopthat/internal/store"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "stopthat"
	serverVersion   = "0.1.0"
)

// Server implements a lightweight stdio Model Context Protocol (MCP) server
// exposing StopThat's scanning, firewall management, and diagnostics as tools.
type Server struct {
	engine firewall.Engine
	store  *store.Store
	scanOS string
}

// NewServer creates an MCP server backed by the given firewall engine and config store.
func NewServer(engine firewall.Engine, st *store.Store, scanOS string) *Server {
	return &Server{engine: engine, store: st, scanOS: scanOS}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeStdio reads newline-delimited JSON-RPC 2.0 messages from r and writes responses to w.
func (s *Server) ServeStdio(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(w)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error: " + err.Error()},
			})
			continue
		}
		// Notifications have no id and expect no response.
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		resp := s.handle(req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handle(req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    serverName,
					"version": serverVersion,
				},
			},
		}
	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": toolDefinitions()},
		}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "invalid params: " + err.Error()},
			}
		}
		result, isErr := s.callTool(p.Name, p.Arguments)
		payload, _ := json.MarshalIndent(result, "", "  ")
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(payload)},
				},
				"isError": isErr,
			},
		}
	default:
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *Server) callTool(name string, rawArgs json.RawMessage) (any, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch name {
	case "stopthat_scan":
		var args struct {
			Preset     string   `json:"preset"`
			ExtraRoots []string `json:"extra_roots"`
			CustomPath string   `json:"custom_path"`
		}
		_ = json.Unmarshal(rawArgs, &args)

		var targets []presets.Target
		if strings.TrimSpace(args.CustomPath) != "" {
			targets = presets.ScanCustomPath(args.CustomPath, s.scanOS)
		} else {
			presetID := strings.TrimSpace(args.Preset)
			if presetID == "" {
				presetID = "adobe"
			}
			extra := append([]string{}, args.ExtraRoots...)
			if s.store != nil {
				if cfg, err := s.store.Load(); err == nil {
					extra = append(extra, cfg.ExtraPresetRoots[presetID]...)
				}
			}
			targets = presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{
				ExtraRoots: extra,
				OS:         s.scanOS,
			})
		}
		rules, _ := s.engine.ListRules(ctx)
		blocked := make(map[string]bool, len(rules))
		for _, r := range rules {
			if r.Enabled {
				blocked[firewall.PathKey(r.ProgramPath)] = true
			}
		}
		type item struct {
			presets.Target
			Blocked bool `json:"blocked"`
		}
		out := make([]item, 0, len(targets))
		for _, t := range targets {
			out = append(out, item{Target: t, Blocked: blocked[firewall.PathKey(t.Path)]})
		}
		return map[string]any{"count": len(out), "targets": out}, false

	case "stopthat_block":
		var args struct {
			Preset            string   `json:"preset"`
			Paths             []string `json:"paths"`
			ExtraRoots        []string `json:"extra_roots"`
			IncludeBackground *bool    `json:"include_background"`
			Outbound          *bool    `json:"outbound"`
			Inbound           *bool    `json:"inbound"`
		}
		_ = json.Unmarshal(rawArgs, &args)
		incBG := true
		if args.IncludeBackground != nil {
			incBG = *args.IncludeBackground
		}
		opts := firewall.DefaultApplyOptions()
		if args.Outbound != nil {
			opts.BlockOutbound = *args.Outbound
		}
		if args.Inbound != nil {
			opts.BlockInbound = *args.Inbound
		}
		var specs []firewall.ProgramSpec
		if len(args.Paths) > 0 {
			for _, p := range args.Paths {
				for _, t := range presets.ScanCustomPath(p, s.scanOS) {
					specs = append(specs, firewall.ProgramSpec{Path: t.Path, DisplayName: t.Name, PresetID: "custom"})
				}
			}
		} else {
			extra := append([]string{}, args.ExtraRoots...)
			if s.store != nil {
				if cfg, err := s.store.Load(); err == nil {
					extra = append(extra, cfg.ExtraPresetRoots["adobe"]...)
				}
			}
			for _, t := range presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{ExtraRoots: extra, OS: s.scanOS}) {
				if !incBG && t.Kind == presets.KindBackground {
					continue
				}
				specs = append(specs, firewall.ProgramSpec{Path: t.Path, DisplayName: t.Name, PresetID: t.PresetID})
			}
		}
		if len(specs) == 0 {
			return map[string]any{"error": "未找到可阻止的可执行文件，请提供 paths 或 extra_roots"}, true
		}
		if err := s.engine.BlockPrograms(ctx, specs, opts); err != nil {
			return map[string]any{"error": err.Error()}, true
		}
		return map[string]any{"blocked_count": len(specs), "options": opts}, false

	case "stopthat_unblock":
		var args struct {
			Paths      []string `json:"paths"`
			ExtraRoots []string `json:"extra_roots"`
		}
		_ = json.Unmarshal(rawArgs, &args)
		paths := args.Paths
		if len(paths) == 0 {
			for _, t := range presets.ScanPreset(presets.AdobePreset(), presets.ScanOptions{ExtraRoots: args.ExtraRoots, OS: s.scanOS}) {
				paths = append(paths, t.Path)
			}
		}
		if err := s.engine.UnblockPrograms(ctx, paths); err != nil {
			return map[string]any{"error": err.Error()}, true
		}
		return map[string]any{"unblocked_paths": len(paths)}, false

	case "stopthat_list_rules":
		rules, err := s.engine.ListRules(ctx)
		if err != nil {
			return map[string]any{"error": err.Error()}, true
		}
		return map[string]any{"count": len(rules), "rules": rules}, false

	case "stopthat_restore_all":
		n, err := s.engine.RemoveAllManaged(ctx)
		if err != nil {
			return map[string]any{"error": err.Error()}, true
		}
		return map[string]any{"removed_rules": n}, false

	case "stopthat_diagnostics":
		return map[string]any{
			"capabilities": s.engine.Capabilities(),
			"diagnostics":  s.engine.Diagnostics(ctx),
		}, false

	case "stopthat_flush_dns":
		if err := s.engine.FlushDNS(ctx); err != nil {
			return map[string]any{"error": err.Error()}, true
		}
		return map[string]any{"ok": true, "message": "DNS 缓存已刷新"}, false

	default:
		return map[string]any{"error": fmt.Sprintf("unknown tool: %s", name)}, true
	}
}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "stopthat_scan",
			"description": "扫描预设应用套件（默认 adobe）或指定路径下的可执行文件及其当前联网拦截状态。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"preset":      map[string]any{"type": "string", "description": "预设套件 ID，目前支持 adobe"},
					"extra_roots": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "额外补充的安装目录"},
					"custom_path": map[string]any{"type": "string", "description": "自定义程序（.exe/.app/.lnk）或目录路径"},
				},
			},
		},
		{
			"name":        "stopthat_block",
			"description": "为预设套件（默认 adobe）或指定路径的程序添加防火墙阻止规则。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"preset":             map[string]any{"type": "string", "description": "预设套件 ID（默认 adobe）"},
					"paths":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "指定要阻止的程序或目录路径列表"},
					"extra_roots":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "预设扫描时额外补充的目录"},
					"include_background": map[string]any{"type": "boolean", "description": "是否同时阻止后台关联组件（默认 true）"},
					"outbound":           map[string]any{"type": "boolean", "description": "阻止出站连接（默认 true）"},
					"inbound":            map[string]any{"type": "boolean", "description": "阻止入站连接（默认 true）"},
				},
			},
		},
		{
			"name":        "stopthat_unblock",
			"description": "恢复指定程序或预设套件的联网权限。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"paths":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "要恢复联网的程序路径列表；为空时恢复 adobe 套件"},
					"extra_roots": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
		},
		{
			"name":        "stopthat_list_rules",
			"description": "列出所有由 StopThat 管理的已生效防火墙规则。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "stopthat_restore_all",
			"description": "一键删除所有由 StopThat 创建的防火墙规则，彻底还原网络设置。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "stopthat_diagnostics",
			"description": "执行网络与防火墙环境体检（管理员权限、系统防火墙、第三方防火墙接管、系统代理状态）。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "stopthat_flush_dns",
			"description": "刷新系统 DNS 缓存。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}
