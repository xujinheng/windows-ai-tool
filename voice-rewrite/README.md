# VoiceRewrite · Windows 本地语音听写 / AI 改写

**一个桌面程序，两颗鼠标侧键，两个工作流。** Windows 11 x64，原生 Go，无 Electron/WebView2/Python 依赖。

> 当前阶段：**Public Beta / 开源准备中**。已通过 Go 单元测试、race 检查、Windows x64 编译；尚未完成干净 Windows 机器安装、微信/VS Code/Chrome 的完整实机验收。

## 30 秒看懂

| 按键 | 怎样使用 | 结果 |
|---|---|---|
| Mouse4 | 把光标放在输入框，按住 Mouse4 讲话、松开 | 原样识别语音，插入光标处 |
| Mouse5 | 选中已有文字，按住 Mouse5 说“翻译成英文”或“润色一下” | 本地 LLM 生成并尝试替换选区 |
| Mouse5 | 选中内容，按住 Mouse5 问“这是什么意思” | 鼠标附近显示解释卡片 |

录音和处理会在屏幕底部显示状态条。两个按键可以取消；退出请右键系统托盘图标。

## 第一次使用

1. 运行 `VoiceRewrite-v16-Windows-x64.exe`。第一次启动自动弹出**设置与连接检测**窗口。以后可在托盘菜单打开。
2. 准备一个 OpenAI 兼容的本地 ASR 服务器，默认 `http://127.0.0.1:8080/v1`，支持 `POST /v1/audio/transcriptions`。推荐 Qwen3-ASR + llama.cpp CUDA 构建。**没有自带 CUDA 二进制、模型文件或安装程序。**
3. 准备 LM Studio 或其他 OpenAI 兼容文本模型服务，默认 `http://127.0.0.1:1234/v1`，加载一个**可处理文本对话**的模型。
4. 在设置中点 ASR 与 LLM 的**测试连接**；确认服务启动后，再尝试 Mouse4，最后测试 Mouse5。
5. 若模型没启动：把对应 CUDA 版 `llama-server.exe`、Qwen3-ASR 主 GGUF、**配套版本**的 `mmproj` GGUF 放在 EXE 同目录或 `models/`，VoiceRewrite 会在端口空闲时尝试启动它，并在退出时只关闭自己启动的进程。需要相应 CUDA DLL 可用。也可继续用已经运行的服务器。

**提示：** `/health` 不存在的 ASR 服务可能仍可转写；设置界面的连接检测不能替代实际听写测试。默认只允许自动启动本机 `127.0.0.1` 服务；不自动启动局域网服务器。

## 模型与设置

GUI 设置支持修改 ASR/LLM API 地址及模型名称、检测连接并保存配置。更多选项通过托盘 → **高级配置 JSON** 打开，配置文件默认位于 EXE 同目录。

```json
{
  "asr_base_url": "http://127.0.0.1:8080/v1",
  "llm_base_url": "http://127.0.0.1:1234/v1",
  "asr_model": "whisper-1",
  "text_model": "auto",
  "dictation_hotkey": "Mouse4",
  "hotkey_record": "Mouse5",
  "auto_start_asr": true,
  "llama_server_path": "",
  "asr_model_path": "",
  "asr_mmproj_path": "",
  "asr_gpu_layers": 99,
  "asr_context_size": 8192
}
```

`llama_server_path`、`asr_model_path`、`asr_mmproj_path` 可指定相对 EXE 目录的文件或绝对路径。留空时在 EXE 同目录及 `models/` 自动查找常见 1.7B/0.6B 文件名。配对错误的 `mmproj` 可能导致转写失败。端口默认 **8080**，无需 OneClick 8081 代理。检测到旧版默认的 `127.0.0.1:8081/v1` 时，程序会自动改成 8080 并尝试保存配置。`auto_start_asr=false` 禁止自动启动。

**OneClick 的语言前缀清理已整合在 VoiceRewrite**，例如 `language English<asr_text>Hello` 自动变成 `Hello`，支持重复标记和 JSON / text/plain ASR 响应。

## 安全、隐私和已知限制

- 录音发送到配置中的 ASR HTTP 端点；选中的文字和识别出的改写指令发送到配置中的 LLM 端点。**这取决于所填服务地址是否为本机或远程，不是无条件的离线保证。**
- 跨应用插入使用系统剪贴板和键盘模拟。未知应用中“已发送粘贴指令”**不等于已确认编辑器成功插入**；写入失败时可从结果卡片复制。
- 原始剪贴板的图片、HTML、RTF 和自定义格式还不能完全恢复；重要剪贴板内容请谨慎使用。
- 使用普通用户权限启动；以管理员权限运行的目标程序可能拒绝模拟输入。
- 请不要在密码框或其他敏感输入框使用该工具。
- 不上传日志或语音的内置云服务；日志在 `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`。

## 开发

`cd voice-rewrite && go test ./... && go test -race ./... && go vet ./...`

交叉编译：`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-H windowsgui -s -w' -o VoiceRewrite.exe .`

更早的开发过程见 `README-V10.md` ～ `README-V16.md`。设计参考 Handy Remote（MIT）及 OpenLess（AGPL，**未复制 OpenLess 源码**），见 `THIRD_PARTY_NOTICES.md`。

**许可证：** 当前仓库的完整开源许可证尚未确定。正式对外宣传为可自由改编/再发布的开源软件前，需要由仓库所有者选择并添加明确的 `LICENSE` 文件。
