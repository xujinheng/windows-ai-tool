# VoiceRewrite · Mouse4 听写 / Mouse5 AI 改写

Windows 11 x64 上的原生 Go 桌面应用，**一个 GUI 管理语音识别和文本模型**。不需要额外安装 Electron、WebView2、Python，也不需要为每个模型手工先开命令行服务。

**注意**：你仍需要提供自己的模型文件、兼容的 `llama-server.exe`（含 GPU/CPU 运行时）或者已经运行的 API 服务。本项目暂未集成模型下载器或 CUDA 推理引擎。

## 操作

| 快捷键 | 做什么 |
| --- | --- |
| 按住 Mouse4 | 对着麦克风说话，松开直接在当前光标输入识别文字 |
| 选中文字，按住 Mouse5 | 说“润色/翻译/修改为……”等指令后优先替换当前选区 |
| 选中文字，按住 Mouse5 提问 | 在附近出现解释/答案卡片 |

浮动状态条不会抢编辑器焦点。粘贴是 Win32 键盘模拟，不能保证所有应用都成功；窗口焦点变更时优先避免误写入其他应用。

## 第一次启动

1. 打开 `VoiceRewrite-v17-Windows-x64.exe`，首次运行会自动弹出「模型与连接」。以后可右键托盘图标打开。
2. 界面上方可选取一个 **`llama-server.exe` 路径**（两个托管模型共用；只用外部 API 就留空）。
3. 在「语音识别 ASR」和「文本处理 LLM」两个标签页**分别**选择：
   - **连接已有服务**：不选“由 VoiceRewrite 自动启动”。填写任意兼容 OpenAI API 的 `.../v1` 地址、模型 ID、必要的 API Key；可为本机、局域网或受信任的云端接口。模型文件不需要在此电脑上。
   - **由 VoiceRewrite 启动**：勾选自动启动。浏览选择本机 ASR GGUF、音频 mmproj（模型需要时），以及 LLM GGUF。两个服务必须用**不同的本地端口**，例如 `8080` 和 `8082`。文本页勾选自动启动时，会自动把默认 `1234` 改为 `8082`。
4. 点击「保存并应用」。托管的 llama-server 将在后台载入模型；等待载入后分别点「测试 ASR」「测试 LLM」。首次加载时间受硬件与模型大小影响。
5. 在记事本等普通输入框里先测试 Mouse4，然后选中文字测试 Mouse5。

所有设置保存在 **`%APPDATA%\VoiceRewrite\config.json`**，无需把程序放在可写目录。UI 无法编辑的 GPU 层数、上下文大小等参数可以通过「高级参数 JSON」修改，重启程序后生效。

### 常见配置范例

**两个用户已有的 API（完全不启动模型进程）：**

```json
{
  "asr_service_mode": "connect",
  "asr_base_url": "http://192.168.1.50:9000/v1",
  "asr_model": "my-speech-model",
  "llm_service_mode": "connect",
  "llm_base_url": "http://localhost:1234/v1",
  "text_model": "my-text-model"
}
```

**两个 GGUF 都由 VoiceRewrite 启动：**

```json
{
  "asr_service_mode": "launch",
  "llm_service_mode": "launch",
  "llama_server_path": "D:\\llama\\llama-server.exe",
  "asr_model_path": "D:\\models\\speech.gguf",
  "asr_mmproj_path": "D:\\models\\speech-mmproj.gguf",
  "llm_model_path": "D:\\models\\general-instruct.gguf",
  "asr_base_url": "http://127.0.0.1:8080/v1",
  "llm_base_url": "http://127.0.0.1:8082/v1",
  "asr_model": "whisper-1",
  "text_model": "auto"
}
```

**混用也可以：**只把 ASR 或 LLM 之一设为 `launch`，另一个设为 `connect`，不受限制。路径可以为完整路径或相对 VoiceRewrite EXE 目录的路径。

**格式限制：** 自启动功能只支持 `llama.cpp` 可以加载的 **GGUF**；ASR 模型还须由所用 `llama-server` 版本支持音频转写 API。单纯放入一个普通文本 GGUF 并不能让它识别音频。对于其他推理框架、API 或模型格式，请选择「连接已有服务」。例如 Qwen3-ASR 的某些 GGUF 需要与主模型匹配的 mmproj 音频投影文件。

**网络安全：** 托管进程仅绑定 `127.0.0.1`。连接模式由用户指定 API 地址，远程地址意味着语音/文本会发到该服务。API Key 保存在当前用户的配置文件中，请注意本机文件权限；不用于任何其他目的。云服务使用者应核查提供方隐私协议。

## 正确理解“单软件”

只需打开 VoiceRewrite 这个桌面 GUI。勾选自启动后，GUI 会自动运行、管理并在退出时结束最多两个 `llama-server.exe` 子进程；**它们仍是后台进程**，只是无需手工管理。连接已有服务时 VoiceRewrite 不会启动或终止用户的服务。

## 诊断和限制

- 两个托管模型可能占用较多 GPU 显存，加载失败可改成较小 GGUF、调低 GPU layers，或只托管一个模型。
- ASR 需要 `POST /v1/audio/transcriptions`，LLM 需要 `POST /v1/chat/completions`；探测连接成功不能保证模型兼容或跨应用粘贴成功。
- 如果按钮测试显示错误，查看 `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`；后台 `llama-server` 输出也写入此日志。
- 密码输入框、提升为管理员的窗口、自定义输入法以及富文本编辑器可能拒绝或异常处理模拟粘贴。不要把跨应用粘贴认为已完全验证。
- 此版未在真实 Windows 桌面执行完整 UI/微信/VS Code/Chrome 验收，测试与编译不等于用户端验收。
- 正式对外开源发布前，还需添加由仓库所有者确认的开源 `LICENSE`、发布构建说明和真实桌面测试结果。

开发：`go test ./... && go test -race ./... && go vet ./...`。Windows 编译：`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .`。
