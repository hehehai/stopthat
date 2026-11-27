---
name: stopthat
description: 使用 StopThat（GUI / CLI / MCP）管理 Windows 与 macOS 本机应用的联网权限。支持自动安装、环境体检、扫描 Adobe 等预设套件或自定义目录、一键阻止/恢复联网、以及一键彻底还原。
---

# StopThat Agent Skill

当用户需要管理本机应用联网权限（例如扫描并管理 Adobe Creative Cloud 套件主程序与后台组件的出站/入站联网、排查防火墙/代理冲突、恢复联网或彻底还原防火墙规则）时，按以下流程操作。

## 1. 检查是否已安装 StopThat

### macOS
```bash
if [ -x /Applications/StopThat.app/Contents/MacOS/StopThat ]; then
  /Applications/StopThat.app/Contents/MacOS/StopThat version
elif command -v stopthat >/dev/null 2>&1; then
  stopthat version
else
  echo "NOT_INSTALLED"
fi
```

### Windows (PowerShell)
```powershell
if (Get-Command stopthat-cli -ErrorAction SilentlyContinue) {
  stopthat-cli version
} elseif (Test-Path "$env:LOCALAPPDATA\StopThat\stopthat-cli.exe") {
  & "$env:LOCALAPPDATA\StopThat\stopthat-cli.exe" version
} else {
  Write-Output "NOT_INSTALLED"
}
```

## 2. 自动下载并安装（如果未安装）

> 所有正式安装包均发布在 GitHub Releases：`https://github.com/hehehai/stopthat/releases/latest`

### macOS（Apple Silicon，已 Developer ID 签名并通过 Apple 公证）
```bash
set -e
TMP_DMG="/tmp/stopthat-latest.dmg"
curl -fL "https://github.com/hehehai/stopthat/releases/latest/download/stopthat-0.1.0-arm64.dmg" -o "$TMP_DMG"
MOUNT_DIR="$(hdiutil attach -nobrowse "$TMP_DMG" | awk '/\/Volumes\// {print substr($0, index($0, "/Volumes/"))}' | tail -n 1)"
rm -rf /Applications/StopThat.app
cp -R "$MOUNT_DIR/StopThat.app" /Applications/StopThat.app
hdiutil detach "$MOUNT_DIR" -quiet
rm -f "$TMP_DMG"
mkdir -p "$HOME/.local/bin"
ln -sf /Applications/StopThat.app/Contents/MacOS/StopThat "$HOME/.local/bin/stopthat"
/Applications/StopThat.app/Contents/MacOS/StopThat version
```

### Windows（PowerShell，安装 GUI + CLI/MCP 双版本）
```powershell
$dir = "$env:LOCALAPPDATA\StopThat"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Invoke-WebRequest -Uri "https://github.com/hehehai/stopthat/releases/latest/download/StopThat-windows-amd64.exe" -OutFile "$dir\StopThat.exe"
Invoke-WebRequest -Uri "https://github.com/hehehai/stopthat/releases/latest/download/stopthat-cli-windows-amd64.exe" -OutFile "$dir\stopthat-cli.exe"
& "$dir\stopthat-cli.exe" version
```

## 3. 标准操作流程（推荐用 `--json` 便于解析）

> 以下 `stopthat` 在 macOS 指 `/Applications/StopThat.app/Contents/MacOS/StopThat`（或 `~/.local/bin/stopthat`），在 Windows 指 `%LOCALAPPDATA%\StopThat\stopthat-cli.exe`（修改 Windows 防火墙需在管理员终端执行）。

1. **先做环境体检**（检查管理员权限、系统防火墙、第三方防火墙接管、代理状态）：
   ```bash
   stopthat doctor --json
   ```
2. **扫描程序**（默认扫描 Adobe 套件；如用户安装在自定义盘符，加 `--dir`）：
   ```bash
   stopthat scan --preset adobe --json
   # 或指定自定义目录：
   stopthat scan --preset adobe --dir "D:\Adobe" --json
   # 或扫描任意非 Adobe 应用/目录：
   stopthat scan --path "C:\Path\To\App" --json
   ```
3. **向用户确认目标列表后执行阻止**（默认同时阻止出站+入站并包含后台组件）：
   ```bash
   stopthat block --preset adobe --json
   # 若只需阻止主程序不含后台组件：
   stopthat block --preset adobe --no-bg --json
   # 若针对自定义路径：
   stopthat block --path "C:\Path\To\App" --json
   ```
4. **验证生效规则**：
   ```bash
   stopthat rules --json
   ```
5. **恢复单个/套件联网或一键全部还原**：
   ```bash
   # 恢复指定程序或套件
   stopthat unblock --preset adobe --json
   # 一键清理所有 StopThat 创建的规则（绝不影响其他防火墙规则）
   stopthat restore --json
   ```
6. **必要时刷新 DNS 缓存**：
   ```bash
   stopthat flush-dns --json
   ```

## 4. MCP（Model Context Protocol）接入方式

如果当前 Agent 支持 MCP（如 WorkBuddy、Claude Desktop、Cursor、OpenClaw 等），可直接将 StopThat 注册为 stdio MCP Server：

### macOS
```json
{
  "mcpServers": {
    "stopthat": {
      "command": "/Applications/StopThat.app/Contents/MacOS/StopThat",
      "args": ["mcp"]
    }
  }
}
```

### Windows
```json
{
  "mcpServers": {
    "stopthat": {
      "command": "C:\\Users\\<你的用户名>\\AppData\\Local\\StopThat\\stopthat-cli.exe",
      "args": ["mcp"]
    }
  }
}
```

可用 MCP 工具清单：
- `stopthat_diagnostics`：网络与防火墙环境体检
- `stopthat_scan`：扫描预设套件或自定义路径
- `stopthat_block`：批量添加防火墙阻止规则
- `stopthat_unblock`：恢复指定程序或套件联网
- `stopthat_list_rules`：查看所有 StopThat 管理的规则
- `stopthat_restore_all`：一键还原全部 StopThat 规则
- `stopthat_flush_dns`：刷新系统 DNS 缓存

## 5. 注意事项
- **Windows**：`block` / `unblock` / `restore` 需要管理员权限；如果 `doctor` 提示检测到火绒/360 等第三方防火墙接管了系统防火墙，应明确告知用户并在对应安全软件中同步放行/拦截。
- **macOS**：系统自带 `socketfilterfw` 仅管理入站连接；如果用户需要 macOS 应用级出站拦截，请根据 `doctor` 输出提醒用户搭配开源的 LuLu 等出站防火墙。
- **安全还原**：卸载前务必先执行 `stopthat restore` 清理全部规则。
