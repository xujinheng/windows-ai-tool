# VoiceRewrite — 纯客户端语音听写与文字改写（v19）

适用于 Windows 11 x64。**不包含模型、不下载模型，也不会启动、停止或配置任何模型推理服务。**

VoiceRewrite 只向你填写的两个 OpenAI 兼容 API 发送请求：

- **语音识别 ASR：** `POST /v1/audio/transcriptions`，收取音频转写文本。
- **文本处理 LLM：** `POST /v1/chat/completions`，用于根据语音指令修改选区或回答问题。

可以连接本机、局域网或远程服务，可以分别使用不同的提供方。模型运行环境由服务提供方负责；这里不做任何 `llama-server.exe`、GGUF、GPU、mmproj 或服务器进程管理。

## 如何使用

1. 运行 `VoiceRewrite-v19-Windows-x64.exe`。首次启动会自动打开「连接设置」；以后右键系统托盘图标选择「设置与连接检测」。
2. **语音识别 API**：填入已在运行的 ASR API 地址（含 `/v1`）、模型 ID，必要时填 API Key。
3. **文本处理 API**：填入已在运行的 LLM API 地址（含 `/v1`）、模型 ID，必要时填 API Key。两组设置在同一页，不用切换标签。
4. 分别点「检测」，然后「保存并应用」。检测只检查 `/models` 是否可访问；部分 ASR 服务没有 `/models`，请用实际听写测试。
5. 在记事本或微信输入框里**按住 Mouse4**，说话后松开，将识别内容插入光标位置；**选中文字并按住 Mouse5** 说“翻译成英文”“简化语句”等，会尝试替换选区；说“这是什么意思”则显示答案卡片。

默认地址示例（**不是软件自动启动的服务器**）：

| 项目 | 默认值 |
| --- | --- |
| ASR API | `http://127.0.0.1:8080/v1` |
| ASR 模型 ID | `whisper-1` |
| LLM API | `http://127.0.0.1:1234/v1` |
| LLM 模型 ID | `auto`，由 API 模型列表自动发现 |

API 路径、模型 ID 和 Key 全部可修改。VoiceRewrite 不指定你必须使用什么模型或推理软件。ASR 接口可以返回 `{"text": "..."}`、JSON 字符串或 `text/plain`。对于 Qwen3-ASR 输出中的 `language Chinese<asr_text>` 等前缀，客户端会直接清理。

## 配置与隐私

设置保存于 `%APPDATA%\VoiceRewrite\config.json`（用户配置目录）。API Key 会存储在此文件内；**不要分享含有真实 Key 的配置文件**。软件只向用户指定的 API 地址发送音频/选中文本，不会自动连接其他模型服务。使用云端服务时应遵守对应服务商的隐私说明。高级配置中的快捷键和时间上限等可以通过「高级 JSON」修改。

本工具对目标编辑器使用剪贴板与键盘模拟。**已发送粘贴指令并不等于已经确认插入成功**；请在不同软件中实测，尤其是富文本编辑器、管理员程序和远程桌面。

## 构建与测试

依赖 Go 1.23+，不需要 CGO、WebView2、Python 或运行时模型库：

```shell
go test ./...
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

Windows 真实桌面测试仍须单独进行。完整的手动验收项目见 `WINDOWS-RELEASE-CHECKLIST.md`。

**开源许可说明：** 仓库所有者尚需确认整个项目的正式 LICENSE。界面参考 Handy，相关说明见 `THIRD_PARTY_NOTICES.md`。
