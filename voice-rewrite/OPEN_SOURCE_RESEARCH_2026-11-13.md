# VoiceRewrite 开源项目调研与工程借鉴（2026-11-13）

> 性质：设计调研 / 研发决策记录，不是运行时代码修复，也不代表已完成 Windows 实机验收。
>
> 核心原则：**跨应用直接改写是主要功能**；划词解释是次要、只读功能。改写明确且源窗口/选区稳定时，优先走直接替换。保护必须围绕写入时的焦点与选区，而不是要求每种编辑框都拥有 Win32 Edit 类名。

## 调研的实际项目与可核对源码

| 项目 | 链接 | 可借鉴要点 | 注意事项 |
| --- | --- | --- | --- |
| WhisPaste | https://github.com/whispaste/whispaste | 全局按住录音、选区语音编辑；独立 orchestrator/state machine；hotkey→STT→paste 集成测试与错误注入 | 并非 Windows 各个外部编辑器自动粘贴都已有 CI 证明；不能将模拟测试当作真机测试 |
| VoiceType AI | https://github.com/devaxl/VoiceType-AI | 保存原前台 HWND、任务 generation、旧任务不允许覆盖新任务、剪贴板快照和失败时改为手动粘贴；静音/幻觉过滤 | 源码 ClipboardGuard 仅恢复 text 或 image，不包含所有富文本格式；对密码控件的检测也只覆盖部分原生控件 |
| FluidVoice Windows | https://github.com/huslermaniac/fluidvoice-windows | 针对普通听写、命令、选区改写有独立操作模式；可在剪贴板粘贴与模拟输入中选择 | 项目自述是早期移植版，尚有未充分验证的组合 |
| TrayTranslator | https://github.com/xianyuzinc/tray-translator | Windows 选区获取多阶段回退；UI Automation TextPattern.GetSelection；Word 特定快路径；剪贴板 IDataObject 快照；非侵入式浮窗 | 核心用途是划词翻译，而非跨应用写回；直接照搬所有 Ctrl+C 回退会增加对用户环境的扰动 |
| Pot | https://github.com/pot-app/pot-desktop | 划词翻译与文本输入、OCR 独立；悬浮结果面板；常用任务有明确入口 | 2026-09-07 归档、GPL-3.0；适合借鉴 UX，不宜作为持续维护的代码依赖 |
| PowerToys Advanced Paste | https://github.com/microsoft/PowerToys | selected-text→custom action→preview/direct paste 两种交付策略；动作与粘贴机制解耦 | 这是对照参考，不是语音改写产品 |
| Textify | https://github.com/m417z/Textify | Windows 经典控件文字获取与 DPI 缩放回退；理解部分目标根本不支持 Ctrl+C | 不要把控件文字读取误当成“该位置可安全替换”的证明 |

阅读过的关键具体文件：

- VoiceType AI: `src-tauri/src/hotkey.rs`, `pipeline.rs`, `winfocus.rs`, `inject.rs`
  - https://github.com/devaxl/VoiceType-AI/blob/main/src-tauri/src/pipeline.rs
  - https://github.com/devaxl/VoiceType-AI/blob/main/src-tauri/src/inject.rs
- TrayTranslator: `Services/SelectionService.cs`, `UI/PopupForm.cs`
  - https://github.com/xianyuzinc/tray-translator/blob/main/Services/SelectionService.cs
  - https://github.com/xianyuzinc/tray-translator/blob/main/UI/PopupForm.cs
- WhisPaste: `ARCHITECTURE.md`, `integration_test/hotkey_to_text_happy_path_test.dart`, `hotkey_to_text_fallback_paths_test.dart`
  - https://github.com/whispaste/whispaste/blob/main/integration_test/hotkey_to_text_fallback_paths_test.dart
- Microsoft API:
  - https://learn.microsoft.com/en-us/windows/win32/winmsg/lowlevelmouseproc
  - https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-threading
  - https://learn.microsoft.com/en-us/windows/desktop/api/ole2/nf-ole2-olegetclipboard

只借鉴设计、测试与行为，不直接复制 GPL/AGPL 项目的源代码；即使 MIT 代码也必须先核对许可和归属。

## 与当前 v8 源码的具体差距（已核对）

以下是 *v8 当前状态*，不是 TODO 已完成：

1. **选区：** `captureSelection` 只读取和保存 CF_UNICODETEXT，`copySelectionWithRetry` 主要依赖模拟 Ctrl+C；未实现 UI Automation 或按应用的选区读取回退。选区失败可能带来 clipboard 改动。
2. **Clipboard：** `replaceSelectionWithVerification` 在写入生成文本后还执行一次 `clipboardCopy` 检查；此操作本身可能修改剪贴板序列号，使原本按 `ownedSeq` 条件恢复的代码失效。原图片/HTML/文件等格式未得到完整保存。
3. **最新任务优先：** worker 处理期间主要由 `app.busy` 阻止新的操作，没有 VoiceType AI 那样的 generation/token 校验；长 LLM 请求的“取消旧操作、只让最新请求写回”机制缺失。
4. **安全写入：** `try_replace` 会比较原选中文字、检查前台 HWND、尝试 Ctrl+V 并做有限事后探测；“未再读到原文字”不构成可靠的写入成功证明，尤其在只读或复杂富文本页面。
5. **端到端：** 有 routeTask / 模拟 HTTP / Win32 小范围窗口测试，但没有可复现的真实桌面三应用替换测试。CI 能编译 Windows 可执行文件，不等于微信/VS Code/Chrome 粘贴成功。
6. **反馈：** 对普通错误有托盘状态与日志，但缺少“结果已复制、未确认插入”的明确中间状态，以及不保存用户真实文字的诊断分级。

## 目标工作流（不改变 Mouse5 默认使用习惯）

```
Mouse5 down -> record -> Mouse5 up -> capture selected text -> local ASR
 -> 1 LLM call (intent + result)
 -> policy router
     explicit edit/transform/translate: write-back pipeline
       [generation valid][same target][same selected text]
       -> guarded paste -> status: success / unverified / failed
       -> if failed/unverified: retain result in copyable card
     explicit question/explain/summary: non-editing card
     ambiguous: non-editing card
```

**规则：**

- **语义不等于写入授权**。明确的语音编辑动词可触发写入尝试；LLM 生成内容不得擅自重定义用户意图。问题/解释不应写回。
- 不把原生 Win32 EDIT 类名当跨应用写回前提；微信、VS Code、Chrome 等很多编辑器只暴露渲染控件。
- 不允许仅凭 Ctrl+C 成功就判断可编辑；“读取选区”“写回验证”应分开设计。
- 每次任务持有 generation、目标窗口（必要时附加进程/控件身份）、选区文本与复制时的 clipboard sequence。
- 用户开启新操作后，旧结果不得再修改新窗口；取消/超时只允许转换到相应终态。
- 不可确认写入成功时不能把状态标成“改写成功”。
- 浮窗仅用于解释、预览和无法确认写入的回退。浮窗需要 ESC/×/按钮关闭、不抢焦点、缩放/DPI/多显示器支持。
- 全局低级钩子只能转发事件，禁止等待 clipboard/HTTP。UI Automation 调用必须在独立适当 COM 线程上执行。
- 剪贴板快照尽量保持多格式原状，并仅在仍拥有本次剪贴板变更时恢复。实现前要评估格式延迟渲染、应用间所有权和剪贴板管理器。
- 不记录用户选中的完整文字、音频或 LLM 全文；仅记录脱敏 app 类型、路由、阶段耗时、失败类型。
- 本地服务仍为默认，不能因调研其他项目而强制安装 Python、Electron、WebView2 或云 API。

## 按重要程度排序的实现路线

### P0 — 不破坏主要功能

1. 先给 v8 建立跨应用测试矩阵（见 `CROSS_APP_ACCEPTANCE.md`），验收微信、VS Code 和 Chrome/Edge 输入框：选中→明确改写→**直接替换**。每次路由更改必须回归这三者。
2. 在 clipboard 层引入原子性的 capture/paste transaction 设计：备份、占用/序列跟踪、写入、明确状态、仅在仍拥有时还原。不要把 `Ctrl+C` 后仍有选区误判为已粘贴。
3. 引入 generation/token，防止旧 worker 的 LLM 结果覆盖最新窗口，拒绝重复录音事件，明确取消语义。

### P1 — 兼容性与降级

4. 参照 TrayTranslator 添加有界、分层的选区读取：快速 Ctrl+C → UIA TextPattern.GetSelection (独立 COM 线程、超时隔离) → 显式“不支持”错误。可后续为 Word/特殊 app 增加适配器。
5. 可配置 app profile：不硬编码单一控件类名；按应用选择 copy/paste/可选模拟输入/只读策略，并显式提醒未知场景的风险。
6. 引入 ASR 空语音/短语幻觉过滤、短重试和阶段性耗时 telemetry（不采集实际文本）。

### P2 — 交互与可维护性

7. 把“已替换”“写入未确认”“仅复制剪贴板”“弹窗回答”明确区分 UI 状态。
8. 结果卡片支持标准关闭、键盘无障碍、DPI/屏幕边缘、简洁小窗/长回答展开。
9. 把适配器、任务路由、剪贴板代理、Windows 真实 UI 测试分离，避免所有职责继续积累在 `windows.go` 中。

## 质量门槛

单元测试负责 intent/route 与 clipboard transaction 状态机；模拟 HTTP 负责接口异常、延迟、JSON 破损；真实 Windows 11 测试负责：

- 微信、VS Code、Chrome/Edge、记事本的真实**选区替换**结果；
- 浏览器静态正文解释后不修改原文；
- 新任务到来后旧结果不得写入；
- 图片/富文本剪贴板不能意外被覆盖；
- 模态窗口、托盘、鼠标 hook 不得因 clipboard/HTTP 超时卡住。

测试证据至少包括 buildID、路由、阶段耗时、写入结果状态（而不是仅显示 `SendInput successful`）。

## 本文记录的实施状态

**调研完成，研发决策与验收矩阵已记录。这里没有修改 v8 的运行时代码，也没有声称这些可靠性优化已经实现。** 下一次真正修改代码时，应先以此文档与验收矩阵为回归基准。
