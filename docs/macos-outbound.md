# macOS 应用级出站拦截扩展说明

## 背景

macOS 系统自带的 `Application Firewall`（`/usr/libexec/ApplicationFirewall/socketfilterfw`）在系统设计上**只管理入站连接（Incoming）**，不拦截应用主动发起的出站连接（Outgoing）。

当前 StopThat 的 macOS 后端（`internal/firewall/engine_darwin.go`）默认使用 `socketfilterfw` 管理应用规则，并在「网络体检」页面检测是否已安装开源/商业出站防火墙（如 LuLu、Little Snitch）以及系统代理状态。

## 后续接入 Apple Network Extension（出站拦截）

如果要在 StopThat 中原生支持 macOS 应用级出站拦截，需要按以下路径扩展：

1. **System Extension + `NEFilterDataProvider`**（Swift 原生插件）：
   - 在 `mygo.json` 中通过 `macos.plugins` 挂载一个 System Extension（`.systemextension`）。
   - 使用 `NEFilterDataProvider` 在 `handleNewFlow(_ flow: NEFilterFlow)` 中读取 `flow.sourceAppAuditToken` → 解析进程路径与签名，按 StopThat 规则表返回 `.drop()` 或 `.allow()`。
2. **Entitlements 与签名分发**：
   - 需要 Apple Developer 账号（Developer ID 分发，Mac App Store 不允许 System Extension 形式的通用防火墙）。
   - 主 App 与 Extension 均需配置：
     - `com.apple.developer.networking.networkextension`（包含 `content-filter-provider-systemextension`）
     - `com.apple.developer.system-extension.install`
   - 使用 Developer ID Application 证书签名并通过 Apple Notarization（公证）。
3. **Go ↔ Extension 通信**：
   - 通过 App Group 共享的 JSON 规则文件或 XPC，将 StopThat 界面里的规则列表同步给 `NEFilterDataProvider`。
