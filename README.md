# StopThat

基于 [MyGo](https://mygo.run/zh) 开发的跨平台（**Windows / macOS**）原生应用联网权限管理工具，同时内置 **图形界面（GUI）+ 命令行（CLI）+ MCP Server**，既支持普通用户一键操作，也支持 **ChatGPT / WorkBuddy / Claude** 等 AI Agent 自动安装并全权代管。

---

## 核心特性

- **内置应用套件预设（首批：Adobe Creative Cloud）**：
  - 自动扫描默认及自定义安装目录，识别 Photoshop、Illustrator、Premiere Pro、After Effects、Lightroom 等**主程序**，以及 `AdobeGCClient`、`AGSService`、`AGMService`、`agcinvokerutility`、`CCXProcess`、`CoreSync` 等**后台关联组件**。
  - 一键全部阻止 / 一键全部恢复 / 单程序独立开关；支持勾选「阻止出站」「阻止入站」「包含后台组件」。
- **通用自定义应用管理**：
  - 支持选择或直接拖入任意 `.exe`、`.app`、Windows `.lnk` 桌面快捷方式或整个安装目录。
  - 一键复制全部扫描到的程序路径清单，方便配合火绒或代理软件使用。
- **安全可逆的规则管理**：
  - 所有规则均带有 `StopThat` 专属前缀，绝不修改或误删系统与用户原有的其他防火墙规则，支持随时**一键全部还原**。
- **网络与防火墙环境体检**：
  - 自动检测管理员权限、系统防火墙开关状态、第三方防火墙接管（火绒 / 360 等）、系统代理状态，并支持一键打开系统防火墙和刷新 DNS 缓存。
- **原生 CLI + MCP（Model Context Protocol）**：
  - 同一套工具既能双击开窗口，也能通过命令行 `--json` 或 stdio MCP 让 AI Agent 自动完成扫描、体检、拦截与恢复。

---

## 一、下载方式

前往 [GitHub Releases](https://github.com/hehehai/stopthat/releases/latest) 下载对应平台版本：

| 平台 | 文件名 | 说明 |
|---|---|---|
| **macOS（Apple Silicon）** | `stopthat-0.1.0-arm64.dmg` | 已通过 **Developer ID 正式签名 + Apple 官方公证**，双击直接安装运行，同时包含 GUI、CLI 与 MCP |
| **Windows（64 位 图形版）** | `StopThat-windows-amd64.exe` | 免安装单文件图形界面版，内置「一键以管理员身份重启」按钮 |
| **Windows（64 位 CLI / MCP 版）** | `stopthat-cli-windows-amd64.exe` | 控制台版，供终端脚本及 AI Agent（CLI / MCP stdio）调用 |

---

## 二、安装方式

### macOS

1. 下载并双击打开 `stopthat-0.1.0-arm64.dmg`。
2. 将 `StopThat.app` 拖入 **`应用程序（/Applications）`** 文件夹。
3. 直接双击启动即可（已通过 Apple 公证，无需绕过 Gatekeeper）。
4. **（可选）启用终端 `stopthat` 命令**：
   ```bash
   mkdir -p ~/.local/bin
   ln -sf /Applications/StopThat.app/Contents/MacOS/StopThat ~/.local/bin/stopthat
   ```

### Windows

1. 下载 `StopThat-windows-amd64.exe`，放到任意固定目录（例如 `C:\Tools\StopThat\` 或 `%LOCALAPPDATA%\StopThat\`）。
2. 双击运行：如果顶部提示需要管理员权限，直接点击横幅上的 **「以管理员身份重启」** 即可弹出 UAC 授权（或右键选择「以管理员身份运行」）。
3. **（可选）如需给终端或 AI Agent 使用 CLI / MCP**：同时下载 `stopthat-cli-windows-amd64.exe` 到同目录即可。

---

## 三、使用方式

### 1. 图形界面（GUI）

启动 StopThat 后，左侧栏分为 4 个功能页：

- **Adobe 套件**：
  - 打开即自动扫描默认目录；若安装在 D 盘等自定义目录，点击 **「添加安装目录」** 或直接把文件夹拖入窗口。
  - 勾选规则选项（推荐保持默认：**阻止出站 + 阻止入站 + 包含后台组件**）。
  - 点击右上角 **「一键全部阻止」** 即可批量写入规则；也可以针对单个程序单独开关。
- **自定义应用**：
  - 点击 **「选择程序」** / **「选择目录」**，或直接把桌面快捷方式（`.lnk`）、`.exe` / `.app`、安装目录拖入窗口即可管理。
- **已生效规则**：
  - 集中查看所有由 StopThat 创建的防火墙规则，支持按条恢复或一键全部还原。
- **网络体检**：
  - 如果发现加了规则仍不生效，先来这里点 **「重新体检」**：
    - 检查是否已获取管理员权限；
    - 检查 Windows 防火墙是否全部开启；
    - 检查是否被火绒/360 等第三方安全软件接管（若已接管，可点击「复制路径清单」到安全软件联网控制中同步拦截）；
    - 检查系统代理状态，并提供 **「打开系统防火墙」** 和 **「刷新 DNS 缓存」** 按钮。

> **平台能力说明**：
> - **Windows**：通过 Windows Defender 防火墙批量管理**出站 + 入站**规则，完整覆盖阻止联网需求。
> - **macOS**：系统自带 `socketfilterfw` 防火墙仅拦截**入站**连接；如需 macOS 应用级**出站**拦截，可配合免费开源的 [LuLu](https://objective-see.org/products/lulu.html) 使用，后续计划通过 Apple Network Extension 原生支持（见 [docs/macos-outbound.md](docs/macos-outbound.md)）。

### 2. 命令行（CLI）

```bash
# 查看环境体检（JSON 输出）
stopthat doctor --json

# 扫描 Adobe 套件（可附加自定义目录）
stopthat scan --preset adobe --dir "D:\Adobe" --json

# 一键阻止 Adobe 套件全部主程序与后台组件
stopthat block --preset adobe --json

# 阻止自定义程序或目录
stopthat block --path "C:\Path\To\MyApp" --json

# 查看当前所有 StopThat 规则
stopthat rules --json

# 刷新 DNS 缓存
stopthat flush-dns --json
```

---

## 四、恢复方式

随时可以将被阻止的程序恢复正常联网，提供 3 种粒度：

1. **恢复单个程序**：在界面列表里关闭该程序右侧的开关，或在 **「已生效规则」** 页面点击对应规则的 **「恢复联网」**。
2. **恢复整个套件**：在 **Adobe 套件** 页面点击 **「一键全部恢复」**，或执行：
   ```bash
   stopthat unblock --preset adobe
   ```
3. **一键全部还原（彻底清除所有 StopThat 规则）**：在 **「已生效规则」** 页面点击 **「一键全部还原」**，或执行：
   ```bash
   stopthat restore
   ```
   > 该操作只会删除带有 `StopThat` 前缀的规则，绝不会影响系统或你手动配置的其他防火墙规则。

---

## 五、卸载方式

为了不残留任何防火墙规则，请按以下顺序卸载：

### 1. 先一键还原全部规则
打开 StopThat → 进入 **「已生效规则」** → 点击 **「一键全部还原」**（或终端运行 `stopthat restore`）。

### 2. 删除程序与配置文件

- **macOS**：
  ```bash
  rm -rf /Applications/StopThat.app
  rm -rf "$HOME/Library/Application Support/StopThat"
  rm -f "$HOME/.local/bin/stopthat"
  ```
- **Windows（PowerShell）**：
  ```powershell
  # 删除你存放 StopThat.exe / stopthat-cli.exe 的目录，以及配置文件目录：
  Remove-Item -Recurse -Force "$env:APPDATA\StopThat" -ErrorAction SilentlyContinue
  Remove-Item -Recurse -Force "$env:LOCALAPPDATA\StopThat" -ErrorAction SilentlyContinue
  ```

---

## 六、AI Agent / Skill & MCP 调用方式

StopThat 原生提供 **`SKILL.md`**、**`--json` CLI** 和 **MCP Server**，可以让 **ChatGPT、WorkBuddy、Claude Code、Cursor、OpenClaw** 等 AI Agent 自动安装并全权帮你完成体检、扫描、拦截和恢复。

### 方式 A：直接把 Skill 链接发给 ChatGPT / WorkBuddy

直接把下面这句话发给具备本机终端/执行能力的 Agent（如 WorkBuddy、Claude Code、Codex 等）：

> 请先阅读这个 Skill 文档：`https://raw.githubusercontent.com/hehehai/stopthat/main/SKILL.md`，按文档自动安装 StopThat，然后帮我做一次网络体检，扫描本机 Adobe 软件并询问我确认后阻止其联网。

Agent 会自动完成：检测环境 → 从 GitHub Releases 下载并安装 StopThat → 运行 `doctor` 体检 → 扫描程序列表 → 经你确认后执行 `block` 或 `restore`。

### 方式 B：接入 MCP Server（WorkBuddy / Claude Desktop / Cursor 等）

在支持 MCP 的客户端配置中加入 `stopthat`（stdio 模式）：

**macOS 配置：**
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

**Windows 配置：**
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

接入后，Agent 可直接调用 7 个标准工具：
- `stopthat_diagnostics`（环境体检）
- `stopthat_scan`（扫描预设套件或自定义路径）
- `stopthat_block`（批量阻止联网）
- `stopthat_unblock`（恢复指定程序/套件联网）
- `stopthat_list_rules`（列出所有生效规则）
- `stopthat_restore_all`（一键还原全部规则）
- `stopthat_flush_dns`（刷新 DNS 缓存）

---

## 本地开发与构建

```bash
# 运行全部单元测试与 MyGo 无头 UI 测试
CGO_ENABLED=0 go test -v ./...

# Windows 交叉编译（GUI 版 + CLI/MCP 版）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o build/StopThat-windows-amd64.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o build/stopthat-cli-windows-amd64.exe .

# macOS 正式签名 + Apple 公证打包（在 Apple Silicon Mac 上运行）
./scripts/release-mac.sh
```

## License

[MIT](./LICENSE)
