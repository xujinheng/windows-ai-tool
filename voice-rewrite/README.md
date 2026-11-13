# VoiceRewrite — 本地 AI 划词助手 (Intent Popup v5)

Windows 11 x64 / Go 标准库，离线优先。继续使用现有本地 Qwen3-ASR OneClick 和 LM Studio 文本模型。无需 Python。

## 怎么使用

- 选中网页文章里的术语，**按住 Mouse5（第二侧键）**说“这是什么意思？”，松开后，在鼠标附近显示 Windows 原生答案小窗。提供“复制答案”“关闭”。
- 选中 Word、浏览器、记事本里的文字，说“总结这段话”“翻译成中文”“为什么这样写”：**弹窗回答，不改原文**。
- 选中**确认可编辑的原生 Win32 Edit/RichEdit 控件**，说“改得简洁一点”：会核验选区并尝试直接替换。
- 对于浏览器的 contenteditable/textarea、网页静态内容、未知或只读控件，即便用户明确说“改写”，也会 **弹出改写预览**。不会猜测能否安全编辑，更不会自动覆盖文章。

快捷键和托盘不变：按住 Mouse5 开始录音，松开提交；托盘右键“编辑 config.json / 查看日志 / 退出 VoiceRewrite”。鼠标的原始前进/后退行为仅在 VoiceRewrite 占用指定侧键时被屏蔽。

## 一次模型调用：意图 + 回答

Qwen3-ASR 先将语音转为文字，随后给文本 LLM **一次** `/v1/chat/completions` 请求（配置 `text_model=auto` 时会额外调用 `/v1/models` 发现模型）。统一 System Prompt 要求输出：

```json
{"intent":"answer","result":"这是该术语的解释……"}
```

意图可以是 `edit`、`answer`、`summarize`、`translate`、`ambiguous`。

**安全边界：** 模型只能建议意图和生成内容；最终是否替换由程序根据语音中的明确修改要求、可编辑控件检测和原选区重新核对来决定。JSON 无效或意图含糊时只弹窗，绝不据此执行自动替换。选中文字被传给模型时只作为不可信参考资料，不执行其中的提示指令。

### 示例 config.json（放在 EXE 同目录）

```json
{
  "asr_base_url": "http://127.0.0.1:8081/v1",
  "llm_base_url": "http://127.0.0.1:1234/v1",
  "text_model": "auto",
  "hotkey_record": "Mouse5",
  "record_mode": "hold",
  "hotkey_exit": "Ctrl+Alt+F10",
  "max_response_tokens": 1200,
  "selection_copy_attempts": 2,
  "selection_copy_timeout_ms": 750,
  "system_prompt": ""
}
```

`config.example.json` 包含完整字段。已有真实 `config.json` 不会被覆盖。自定义非空 `system_prompt` 必须要求模型按统一 JSON 格式输出，否则只能回退到安全弹窗。不要把真实密钥和个人电脑路径提交到 GitHub。

## 安装与验证

1. 确认本地 ASR `127.0.0.1:8081/v1` 和 LM Studio `127.0.0.1:1234/v1` 已运行，且 LM Studio 加载了**文本 Instruct 模型**，不是 Qwen3-ASR。
2. 完全退出旧版 VoiceRewrite。新 EXE 放到已有 `config.json` 的目录，或者使用全新目录首次运行并自动生成配置。
3. 先测试**网页非编辑文字的提问弹窗**，再测试**记事本原生 Edit 文本控件的改写替换**。
4. 查看日志 `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`，确认启动标记 `build=2026-11-13-intent-popup-v5`。日志只写请求阶段、意图、字符数和耗时，不记录原文/语音内容。

## 已知限制

- **Windows 11 实机仍需验证。** Linux 的 Go 测试和 Windows 交叉编译不等于鼠标、窗口、中文输入法都已经测试成功。
- 跨应用选区仍依赖模拟 `Ctrl+C`，因此有些网页、受保护窗口或管理员权限程序可能读不到选区；程序不会擅自修改其他窗口。
- 本版编辑自动替换仅支持可验证的原生 Windows Edit/RichEdit 控件，浏览器编辑框会先弹出预览。后续可通过 UI Automation 扩展兼容范围。
- 弹窗显示文字最多约 5500 字，较长答案会标明截断；可以在 `config.json` 里调整 `max_response_tokens`（128–4096）。
- 现有剪贴板方案仅尝试恢复**纯文本格式**；图片、文件和富文本等剪贴板格式尚不能完整恢复。请勿在依赖非文本剪贴板时使用自动替换。
- 当前并未实现文档检索、网页事实核验或多轮对话，模型可能出现事实性错误；显示解释不意味着进行了外部查询。

详见 [ARCHITECTURE.md](ARCHITECTURE.md) 和 [MANUAL-TESTS.md](MANUAL-TESTS.md)。
