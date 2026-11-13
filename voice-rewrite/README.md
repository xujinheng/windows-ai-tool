## v8: edit-first cross-application behavior (2026-11-13)

**Direct replacement is the primary feature.** For clear rewrite, polish,
correction, abbreviation or translation commands, the app attempts the original
cross-application Ctrl+C -> LLM -> verify selected text -> Ctrl+V workflow,
including WeChat, VS Code, Chrome/Edge and other desktop editors. It must NOT
require a native Win32 EDIT class, since browser/Electron controls lack one.

Questions and explanations ("这是什么意思", "为什么") and summaries still display
an answer card. An explicit transformation incorrectly labeled as an answer by
the model triggers an edit-only LLM retry rather than pasting an explanation.
Unknown controls receive an additional post-paste no-op probe; this is best-effort,
not a 100% reliable read-only check. If selection changed or paste appears to do
nothing, the result is displayed in a copyable card instead of overwriting a
potentially unrelated window.

Build ID: `2026-11-13-edit-first-v8`. Config remains unchanged. **Close all
older VoiceRewrite processes before starting**: multiple mouse hooks will fight.

# VoiceRewrite v6 — 语音改写 + 划词问答

Windows 11 x64 / Go 标准库 / 离线优先。默认 **Mouse5 按住录音、松开提交**，使用现有 Qwen3-ASR OneClick 与 LM Studio 文本模型。

### v6 相比 v5 修复了什么

- **弹窗空白/不能关闭**：主 Windows 消息队列现在会调用 `TranslateMessage` / `DispatchMessageW`，确保 `WM_PAINT`、`WM_CLOSE` 与按钮 `WM_COMMAND` 能送达窗口过程。编辑框的内容长度还会在创建后校验。
- **关闭方式**：窗口右上角 ×、底部「关闭」、`Esc`（当结果窗口获得焦点）、「复制并关闭」、托盘右键「关闭 AI 结果窗口」。
- **翻译误路由**：用户说「写成英文」「翻译成中文」，模型一次返回 `intent=translate` + 完整译文；可确认是原生可编辑控件时自动替换。网页正文及无法确认可编辑的应用只显示译文预览，不会尝试危险的粘贴。
- **结果卡片**：短回答采用更小的窗口，长回答增高；可复制，不抢走输入焦点。

### 示例

| 用户口令 | 目标 | 结果 |
|---|---|---|
| 改得更简洁 | 可确认可编辑的 Win32 Edit/RichEdit | 替换原文 |
| 请写成英文 | 可确认可编辑的 Win32 Edit/RichEdit | 替换为英文 |
| 请写成英文 | 浏览器正文、只读内容、无法确认可编辑的控件 | 弹出翻译并提供复制 |
| 这是什么意思 | 任意选中文字 | 弹出解释，不改原文 |
| 总结这段话 | 任意选中文字 | 弹出摘要，不改原文 |

现阶段为了避免在网页正文误粘贴，自动替换仅支持能够确认的原生可编辑控件。**浏览器输入框和部分第三方编辑器仍可能只能获得弹窗预览**，这是有意保留的安全限制，不是承诺支持所有编辑器。

### 安装和日志

退出旧版 VoiceRewrite，把新版 EXE 和原来的 `config.json` 放在同一文件夹。保留 `record_mode=hold`、`hotkey_record=Mouse5`；默认本地 ASR 地址 `http://127.0.0.1:8081/v1`，本地文本模型 `http://127.0.0.1:1234/v1`。原有 `config.json` 不会被自动覆盖；自定义 `system_prompt` 会覆盖统一意图提示词，想使用新版统一 Prompt 请留空。

启动日志版本 `build=2026-11-13-intent-popup-v6`。PowerShell：

```powershell
Get-Content "$env:LOCALAPPDATA\VoiceRewrite\VoiceRewrite.log" -Encoding UTF8 -Tail 45
```

#### 测试

- Linux Go tests: `go test ./...`, `go test -race ./...`, `go vet ./...`。
- Windows x64: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`。
- 真正窗口关闭/绘制需在 Windows 交互桌面验证：

```powershell
$env:VOICE_REWRITE_NATIVE_POPUP_TEST='1'
go test -run TestNativePopupPaintAndClose -v
```

此版本没有在用户的真实 Windows 11 桌面进行测试，Windows 交互行为需本机验收。

## v7: modern result card and browser text transformations

- Build ID: `2026-11-13-modern-card-v7`; default Mouse5 hold-to-talk is unchanged.
- Explicit editing/translating instructions in verified native Win32 Edit/RichEdit controls use the existing checked replacement path.
- For Chromium/Firefox/Electron rendering windows, an explicit edit/translate instruction can try a **guarded browser replacement**. The originally selected text and foreground window are verified before pasting. A second clipboard check detects unchanged selections (a common sign of a read-only page); then the result is shown as a preview. **This is a heuristic, not a guaranteed browser editability detector.** Use it on non-sensitive text and keep undo available.
- For questions, explanations, summaries, uncertain instructions, and any unsuccessful replacement, show a near-cursor card with white background, a Windows 11 rounded frame, readable typography, scrollable text and custom-drawn buttons.
- The card shows `应用到原文` only for transformations. Clicking it restores focus to the original window, rechecks the original selection on a worker and tries to apply; failure leaves the preview visible. Other actions: `复制结果`, `关闭`, `Esc`, and the tray's `关闭 AI 结果窗口`.
- DWM rounding uses native Windows 11 APIs. Windows desktop tests are still required for Chrome rich editors, monitor DPI variations, and mouse focus changes. No browser extension or WebView2 is installed.
