# Privacy

VoiceRewrite sends data to the ASR and LLM endpoints configured in Settings. If you configure a remote endpoint, its operator receives the relevant data.

| Data | Use | Local storage |
| --- | --- | --- |
| Microphone audio | Speech recognition | In memory during the task |
| Selected text and spoken instructions | Rewriting, translation and questions | In memory during the task |
| API keys | API authentication | Windows DPAPI-encrypted configuration |
| Endpoints, models and shortcuts | Application settings | `%APPDATA%\VoiceRewrite` |
| Recent text results (up to 10) | Session recovery | Memory only |
| Clipboard snapshot | Restore clipboard data during insertion | Memory only |
| Diagnostic logs | Troubleshooting | `%LOCALAPPDATA%\VoiceRewrite` |

Mouse4/Mouse5 presses may briefly open the microphone while the app determines whether the button is held. Short-click audio is discarded in memory without being sent to a server or saved as a result.

The tray includes a **Clear session history** command. Exiting VoiceRewrite also clears the in-memory results. Technical logs rotate at 4 MiB with three backups; audio and selection content are not intentionally logged.

The ASR connection test sends generated silent audio to the configured endpoint. Update checking contacts GitHub only when requested from the tray.

To delete locally stored settings or logs, exit the app and remove their respective directories. The uninstaller preserves user settings.

Automatic text insertion and password-field detection depend on Windows and the target editor. See [Security](SECURITY.md).
