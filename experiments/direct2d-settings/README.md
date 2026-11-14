# VoiceRewrite · Direct2D / DirectWrite 原生设置窗口试验

这是 **独立 Windows x64 原型**，不修改 VoiceRewrite 正式的 Go/Win32 设置窗口。没有 Node.js、Electron、WebView2、Wails 或第三方 Go 依赖。

- 画布、圆角、导航、高亮、边框：`ID2D1HwndRenderTarget` (Direct2D)
- 中文标题、段落：`IDWriteTextFormat` + `DrawText` (DirectWrite)
- 实际可编辑输入与麦克风下拉框：原生 Win32 `EDIT` / `COMBOBOX`
- 支持深浅主题、三个克制的分类、鼠标切换、Tab 键、窗口缩放、DPI 比例参数
- “保存设置”只是演示，不向现有配置写入任何值；地址和模型均为示例

## 本地构建

Windows x64、Go 1.23+：

```powershell
cd experiments/direct2d-settings
go build -trimpath -o Direct2D-Settings.exe .
.\Direct2D-Settings.exe
```

可以用环境变量 `VOICEREWRITE_D2D_THEME=light` 预览浅色主题，以及 `VOICEREWRITE_D2D_SCALE=1.25` 模拟更高 DPI。两个变量仅用于独立原型。

## 实测方式

Windows CI 启动实际 EXE、截取窗口，并记录 `WorkingSet64` 和 `PrivateMemorySize64`。请注意：**这是原型进程的内存，不是已存在的 40.4 MB 正式 VoiceRewrite.exe 的增量**。对比前应在同一 Windows 设备上测量原版和实际集成后的版本，取稳定空闲时的进程工作集或私有字节，避免混用统计口径。GPU/共享图形驱动资源可能没有包含在这些数字内。

## 工程边界

此试验验证技术路径，不代表可以直接替换主程序：安全保存与原有配置系统、麦克风枚举、探测网络服务、输入法焦点、屏幕阅读器、高 DPI 显示器切换和绘制资源生命周期均需要真实集成时逐项验收。COM vtable 调用限定 Windows amd64 构建，不能直接移植到 32 位 Windows。