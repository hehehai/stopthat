# StopThat

基于 [MyGo](https://mygo.run/zh) 开发的跨平台（Windows / macOS）原生应用联网管理工具。

## 功能定位

- **应用联网权限管理**：统一查看并一键开关本机应用的出站/入站联网权限。
- **内置应用套件预设（首批：Adobe Creative Cloud）**：
  - 自动扫描默认安装目录及用户补充目录，识别 Photoshop、Illustrator、Premiere Pro、After Effects、Lightroom 等主程序，以及 `AdobeGCClient`、`AGSService`、`AGMService`、`agcinvokerutility` 等关联后台组件。
  - 支持一键全部阻止 / 一键全部恢复 / 按程序单独开关，支持「同时阻止出站和入站」与「包含后台组件」选项。
- **自定义应用与目录**：支持选择或粘贴任意 `.exe` / `.app` / 安装目录（自动去除 Windows 快捷方式路径首尾双引号）。
- **专属规则命名空间**：所有由 StopThat 创建的防火墙规则均带有 `StopThat` 前缀，绝不误删系统或用户原有规则，支持随时一键彻底还原。
- **网络与防火墙环境体检**：检测管理员权限、系统防火墙开关状态、第三方防火墙接管（如火绒/360 等）、系统代理状态，并提供一键刷新 DNS 缓存。

## 平台能力说明

- **Windows**：通过 Windows Defender 防火墙管理按进程路径的**出站 + 入站**阻止规则（需右键「以管理员身份运行」）。
- **macOS**：通过系统应用防火墙（`socketfilterfw`）管理应用入站规则；同时在体检页明确提示 macOS 自带防火墙仅支持入站，如需应用级出站拦截可后续扩展 Apple Network Extension 或搭配开源的 LuLu。

## 本地开发与构建

```bash
# 运行全部单元测试与 MyGo 无头 UI 测试
go test ./...

# 交叉编译 Windows / macOS（纯 Go，无需 cgo）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o build/StopThat.exe .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o build/StopThat-darwin-arm64 .
```
