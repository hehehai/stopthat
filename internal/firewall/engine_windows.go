//go:build windows

package firewall

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type windowsEngine struct{}

// NewEngine returns the Windows Defender Firewall backend.
func NewEngine() Engine {
	return &windowsEngine{}
}

func (e *windowsEngine) Capabilities() Capabilities {
	elevated := isWindowsAdmin()
	hint := ""
	if !elevated {
		hint = "当前未以管理员身份运行，修改 Windows 防火墙规则需要右键「以管理员身份运行」。"
	}
	return Capabilities{
		Platform:           "windows",
		EngineName:         "Windows Defender 防火墙",
		SupportsOutbound:   true,
		SupportsInbound:    true,
		Elevated:           elevated,
		NeedsElevationHint: hint,
	}
}

func (e *windowsEngine) ListRules(ctx context.Context) ([]Rule, error) {
	// Use PowerShell to reliably query rules whose DisplayName starts with "StopThat ".
	ps := `Get-NetFirewallRule -DisplayName 'StopThat *' -ErrorAction SilentlyContinue | ForEach-Object { $f = $_ | Get-NetFirewallApplicationFilter -ErrorAction SilentlyContinue; "$($_.DisplayName)` + "\t" + `$($_.Direction)` + "\t" + `$($_.Enabled)` + "\t" + `$($f.Program)" }`
	out, err := runHidden(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		// Fallback to netsh if PowerShell is restricted.
		return e.listRulesNetsh(ctx)
	}
	var rules []Rule
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		dirRaw := strings.ToLower(strings.TrimSpace(parts[1]))
		enabledRaw := strings.ToLower(strings.TrimSpace(parts[2]))
		prog := NormalizePath(parts[3])
		if prog == "" || prog == "any" {
			continue
		}
		dir := DirOut
		if strings.HasPrefix(dirRaw, "in") {
			dir = DirIn
		}
		rules = append(rules, Rule{
			Name:        name,
			ProgramPath: prog,
			Direction:   dir,
			DisplayName: extractDisplayName(name),
			Enabled:     enabledRaw == "true",
		})
	}
	return rules, nil
}

func (e *windowsEngine) listRulesNetsh(ctx context.Context) ([]Rule, error) {
	out, err := runHidden(ctx, "netsh", "advfirewall", "firewall", "show", "rule", "name=all", "verbose")
	if err != nil {
		return nil, fmt.Errorf("查询防火墙规则失败: %w", err)
	}
	var rules []Rule
	var cur *Rule
	flush := func() {
		if cur != nil && strings.HasPrefix(cur.Name, RulePrefix+" ") && cur.ProgramPath != "" {
			rules = append(rules, *cur)
		}
		cur = nil
	}
	for _, raw := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(line[:idx]))
		v := strings.TrimSpace(line[idx+1:])
		switch {
		case k == "rule name" || k == "规则名称":
			flush()
			cur = &Rule{Name: v, DisplayName: extractDisplayName(v), Enabled: true}
		case cur != nil && (k == "direction" || k == "方向"):
			if strings.EqualFold(v, "in") || strings.Contains(v, "入") {
				cur.Direction = DirIn
			} else {
				cur.Direction = DirOut
			}
		case cur != nil && (k == "program" || k == "程序"):
			if !strings.EqualFold(v, "any") && v != "任何" {
				cur.ProgramPath = NormalizePath(v)
			}
		case cur != nil && (k == "enabled" || k == "已启用"):
			cur.Enabled = strings.EqualFold(v, "yes") || v == "是"
		}
	}
	flush()
	return rules, nil
}

func (e *windowsEngine) BlockPrograms(ctx context.Context, programs []ProgramSpec, opts ApplyOptions) error {
	if !opts.BlockOutbound && !opts.BlockInbound {
		return nil
	}
	if !isWindowsAdmin() {
		return fmt.Errorf("需要管理员权限：请右键 StopThat 选择「以管理员身份运行」")
	}
	for _, p := range programs {
		progPath := NormalizePath(p.Path)
		if progPath == "" {
			continue
		}
		if opts.BlockOutbound {
			name := RuleName(DirOut, progPath, p.DisplayName)
			_, _ = runHidden(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
			if _, err := runHidden(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
				"name="+name,
				"dir=out",
				"action=block",
				"program="+progPath,
				"profile=any",
				"enable=yes",
			); err != nil {
				return fmt.Errorf("添加出站规则失败 (%s): %w", p.DisplayName, err)
			}
		}
		if opts.BlockInbound {
			name := RuleName(DirIn, progPath, p.DisplayName)
			_, _ = runHidden(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
			if _, err := runHidden(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
				"name="+name,
				"dir=in",
				"action=block",
				"program="+progPath,
				"profile=any",
				"enable=yes",
			); err != nil {
				return fmt.Errorf("添加入站规则失败 (%s): %w", p.DisplayName, err)
			}
		}
	}
	return nil
}

func (e *windowsEngine) UnblockPrograms(ctx context.Context, paths []string) error {
	if !isWindowsAdmin() {
		return fmt.Errorf("需要管理员权限：请右键 StopThat 选择「以管理员身份运行」")
	}
	rules, err := e.ListRules(ctx)
	if err != nil {
		return err
	}
	wanted := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		wanted[PathKey(p)] = struct{}{}
	}
	for _, r := range rules {
		if _, ok := wanted[PathKey(r.ProgramPath)]; ok {
			if _, err := runHidden(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name="+r.Name); err != nil {
				return fmt.Errorf("删除规则失败 (%s): %w", r.Name, err)
			}
		}
	}
	return nil
}

func (e *windowsEngine) RemoveAllManaged(ctx context.Context) (int, error) {
	if !isWindowsAdmin() {
		return 0, fmt.Errorf("需要管理员权限：请右键 StopThat 选择「以管理员身份运行」")
	}
	rules, err := e.ListRules(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, r := range rules {
		if _, err := runHidden(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name="+r.Name); err == nil {
			removed++
		}
	}
	return removed, nil
}

func (e *windowsEngine) Diagnostics(ctx context.Context) []DiagnosticItem {
	var items []DiagnosticItem

	// 1. Admin check
	if isWindowsAdmin() {
		items = append(items, DiagnosticItem{
			Title:  "管理员权限",
			Status: "已获取",
			Detail: "当前进程具有管理员权限，可直接写入和清理 Windows 防火墙规则。",
			Level:  DiagOK,
		})
	} else {
		items = append(items, DiagnosticItem{
			Title:  "管理员权限",
			Status: "未获取",
			Detail: "请右键 StopThat 选择「以管理员身份运行」，否则无法写入防火墙规则。",
			Level:  DiagError,
		})
	}

	// 2. Windows Firewall profile status
	fwOut, fwErr := runHidden(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-NetFirewallProfile | ForEach-Object { "$($_.Name)=$($_.Enabled)" }`)
	if fwErr != nil {
		items = append(items, DiagnosticItem{
			Title:  "Windows Defender 防火墙",
			Status: "检测失败",
			Detail: "无法读取防火墙配置文件状态，请确认 Windows Defender Firewall 服务（MpsSvc）已启动。",
			Level:  DiagWarn,
		})
	} else {
		lines := strings.Fields(fwOut)
		disabled := 0
		for _, kv := range lines {
			if strings.HasSuffix(strings.ToLower(kv), "=false") {
				disabled++
			}
		}
		if disabled == 0 {
			items = append(items, DiagnosticItem{
				Title:  "Windows Defender 防火墙",
				Status: "全部已开启",
				Detail: "域、专用、公用网络防火墙均已启用：" + strings.Join(lines, " / "),
				Level:  DiagOK,
			})
		} else {
			items = append(items, DiagnosticItem{
				Title:  "Windows Defender 防火墙",
				Status: "部分未开启",
				Detail: "检测到防火墙未全部开启（" + strings.Join(lines, " / ") + "），规则可能不会生效。",
				Level:  DiagWarn,
			})
		}
	}

	// 3. Third-party security / firewall products
	avOut, avErr := runHidden(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance -Namespace root/SecurityCenter2 -ClassName FirewallProduct -ErrorAction SilentlyContinue | Select-Object -ExpandProperty displayName`)
	if avErr == nil && strings.TrimSpace(avOut) != "" {
		names := strings.Join(strings.Split(strings.TrimSpace(strings.ReplaceAll(avOut, "\r\n", "\n")), "\n"), "、")
		items = append(items, DiagnosticItem{
			Title:  "第三方防火墙接管检测",
			Status: "发现第三方产品",
			Detail: "检测到已注册的第三方防火墙：" + names + "。若系统防火墙已被接管，请在对应安全软件的「联网控制」中同步拦截。",
			Level:  DiagWarn,
		})
	} else {
		items = append(items, DiagnosticItem{
			Title:  "第三方防火墙接管检测",
			Status: "无冲突",
			Detail: "未检测到接管 Windows 防火墙的第三方防火墙产品。",
			Level:  DiagOK,
		})
	}

	// 4. System proxy check
	proxyOut, proxyErr := runHidden(ctx, "reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`, "/v", "ProxyEnable")
	if proxyErr == nil && strings.Contains(strings.ToLower(proxyOut), "0x1") {
		items = append(items, DiagnosticItem{
			Title:  "系统代理检测",
			Status: "已开启代理",
			Detail: "检测到系统代理已开启。若使用代理软件（Clash/v2ray 等）的 TUN 或规则分流模式，请确认未绕过本机防火墙规则。",
			Level:  DiagWarn,
		})
	} else {
		items = append(items, DiagnosticItem{
			Title:  "系统代理检测",
			Status: "未开启",
			Detail: "当前未开启系统级 HTTP/SOCKS 代理。",
			Level:  DiagOK,
		})
	}

	return items
}

func (e *windowsEngine) FlushDNS(ctx context.Context) error {
	_, err := runHidden(ctx, "ipconfig", "/flushdns")
	if err != nil {
		return fmt.Errorf("刷新 DNS 缓存失败: %w", err)
	}
	return nil
}

func isWindowsAdmin() bool {
	f, err := os.Open(`\\.\PHYSICALDRIVE0`)
	if err == nil {
		_ = f.Close()
		return true
	}
	return false
}

func runHidden(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
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

func extractDisplayName(ruleName string) string {
	// Format: StopThat [out] <hash> <displayName>
	parts := strings.SplitN(ruleName, " ", 4)
	if len(parts) == 4 {
		return parts[3]
	}
	return ruleName
}
