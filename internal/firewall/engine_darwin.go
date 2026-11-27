//go:build darwin

package firewall

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const socketFilterFW = "/usr/libexec/ApplicationFirewall/socketfilterfw"

type darwinEngine struct{}

// NewEngine returns the macOS Application Firewall (ALF) backend.
func NewEngine() Engine {
	return &darwinEngine{}
}

func (e *darwinEngine) Capabilities() Capabilities {
	return Capabilities{
		Platform:         "darwin",
		EngineName:       "macOS 应用防火墙 (socketfilterfw)",
		SupportsOutbound: false,
		SupportsInbound:  true,
		Elevated:         os.Geteuid() == 0,
		OutboundNote:     "提示：macOS 自带防火墙（socketfilterfw）仅拦截入站连接；如需系统级按进程拦截出站连接，需集成 Apple Network Extension 签名扩展，或搭配开源出站防火墙（如 LuLu）。",
	}
}

func (e *darwinEngine) ListRules(ctx context.Context) ([]Rule, error) {
	out, err := runCmd(ctx, socketFilterFW, "--listapps")
	if err != nil {
		return nil, fmt.Errorf("读取 macOS 应用防火墙列表失败: %w", err)
	}
	var rules []Rule
	var curPath string
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if idx := strings.Index(line, " : "); idx > 0 {
			p := strings.TrimSpace(line[idx+3:])
			if strings.HasPrefix(p, "/") {
				curPath = NormalizePath(p)
			}
			continue
		}
		if curPath != "" && strings.Contains(strings.ToLower(line), "block incoming connections") {
			rules = append(rules, Rule{
				Name:        RuleName(DirIn, curPath, ""),
				ProgramPath: curPath,
				Direction:   DirIn,
				DisplayName: baseAppName(curPath),
				Enabled:     true,
			})
			curPath = ""
		} else if curPath != "" && strings.Contains(strings.ToLower(line), "allow incoming connections") {
			curPath = ""
		}
	}
	return rules, nil
}

func (e *darwinEngine) BlockPrograms(ctx context.Context, programs []ProgramSpec, opts ApplyOptions) error {
	if len(programs) == 0 {
		return nil
	}
	var cmds []string
	// Ensure macOS Application Firewall is enabled.
	cmds = append(cmds, fmt.Sprintf("%q --setglobalstate on", socketFilterFW))
	for _, p := range programs {
		progPath := NormalizePath(p.Path)
		if progPath == "" {
			continue
		}
		cmds = append(cmds,
			fmt.Sprintf("%q --add %q", socketFilterFW, progPath),
			fmt.Sprintf("%q --blockapp %q", socketFilterFW, progPath),
		)
	}
	return runDarwinPrivileged(ctx, strings.Join(cmds, " && "))
}

func (e *darwinEngine) UnblockPrograms(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	var cmds []string
	for _, p := range paths {
		progPath := NormalizePath(p)
		if progPath == "" {
			continue
		}
		cmds = append(cmds, fmt.Sprintf("%q --remove %q", socketFilterFW, progPath))
	}
	return runDarwinPrivileged(ctx, strings.Join(cmds, " && "))
}

func (e *darwinEngine) RemoveAllManaged(ctx context.Context) (int, error) {
	rules, err := e.ListRules(ctx)
	if err != nil {
		return 0, err
	}
	if len(rules) == 0 {
		return 0, nil
	}
	paths := make([]string, 0, len(rules))
	for _, r := range rules {
		paths = append(paths, r.ProgramPath)
	}
	if err := e.UnblockPrograms(ctx, paths); err != nil {
		return 0, err
	}
	return len(paths), nil
}

func (e *darwinEngine) Diagnostics(ctx context.Context) []DiagnosticItem {
	var items []DiagnosticItem

	// 1. macOS Application Firewall state
	out, err := runCmd(ctx, socketFilterFW, "--getglobalstate")
	if err != nil {
		items = append(items, DiagnosticItem{
			Title:  "macOS 应用防火墙 (ALF)",
			Status: "检测失败",
			Detail: err.Error(),
			Level:  DiagWarn,
		})
	} else if strings.Contains(strings.ToLower(out), "enabled") {
		items = append(items, DiagnosticItem{
			Title:  "macOS 应用防火墙 (ALF)",
			Status: "已开启",
			Detail: "系统应用防火墙已启用，可正常管理应用入站规则。",
			Level:  DiagOK,
		})
	} else {
		items = append(items, DiagnosticItem{
			Title:  "macOS 应用防火墙 (ALF)",
			Status: "未开启",
			Detail: "系统应用防火墙当前关闭；首次执行拦截时 StopThat 会自动提示并开启。",
			Level:  DiagWarn,
		})
	}

	// 2. Outbound firewall presence (LuLu / Little Snitch)
	hasLuLu := fileExists("/Applications/LuLu.app")
	hasLS := fileExists("/Applications/Little Snitch.app")
	switch {
	case hasLuLu || hasLS:
		found := "LuLu"
		if hasLS && hasLuLu {
			found = "LuLu、Little Snitch"
		} else if hasLS {
			found = "Little Snitch"
		}
		items = append(items, DiagnosticItem{
			Title:  "出站拦截能力（macOS）",
			Status: "已检测到 " + found,
			Detail: "macOS 系统自带防火墙仅管入站；你已安装 " + found + "，可实现应用级出站联网拦截。",
			Level:  DiagOK,
		})
	default:
		items = append(items, DiagnosticItem{
			Title:  "出站拦截能力（macOS）",
			Status: "仅支持入站",
			Detail: "macOS 系统自带 socketfilterfw 仅管入站不管出站。如需彻底阻止应用出站联网，建议后续集成 Apple Network Extension 或搭配免费开源的 LuLu。",
			Level:  DiagInfo,
		})
	}

	// 3. System proxy state
	proxyOut, proxyErr := runCmd(ctx, "scutil", "--proxy")
	if proxyErr == nil && (strings.Contains(proxyOut, "HTTPEnable : 1") ||
		strings.Contains(proxyOut, "HTTPSEnable : 1") ||
		strings.Contains(proxyOut, "SOCKSEnable : 1")) {
		items = append(items, DiagnosticItem{
			Title:  "系统代理检测",
			Status: "已开启代理",
			Detail: "检测到 macOS 已启用系统代理。代理软件的分流/TUN 规则可能影响本机网络拦截策略。",
			Level:  DiagWarn,
		})
	} else {
		items = append(items, DiagnosticItem{
			Title:  "系统代理检测",
			Status: "未开启",
			Detail: "当前未启用系统级 HTTP/HTTPS/SOCKS 代理。",
			Level:  DiagOK,
		})
	}

	return items
}

func (e *darwinEngine) FlushDNS(ctx context.Context) error {
	if _, err := runCmd(ctx, "dscacheutil", "-flushcache"); err != nil {
		return fmt.Errorf("刷新 DNS 缓存失败: %w", err)
	}
	return nil
}

func runDarwinPrivileged(ctx context.Context, shellCmd string) error {
	if os.Geteuid() == 0 {
		_, err := runCmd(ctx, "/bin/sh", "-c", shellCmd)
		return err
	}
	escaped := strings.ReplaceAll(shellCmd, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escaped)
	_, err := runCmd(ctx, "osascript", "-e", script)
	if err != nil {
		return fmt.Errorf("管理员授权执行失败: %w", err)
	}
	return nil
}

func runCmd(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return stdout.String(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func baseAppName(p string) string {
	parts := strings.Split(p, "/")
	for _, seg := range parts {
		if strings.HasSuffix(seg, ".app") {
			return strings.TrimSuffix(seg, ".app")
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return p
}
