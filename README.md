# ⚡ cc-notify

**Windows desktop notifications for [Codex CLI](https://github.com/openai/codex) and [Claude Code](https://docs.anthropic.com/en/docs/claude-code).**

Get a toast notification or popup dialog whenever Codex or Claude Code finishes a task — even when the terminal is in the background.

## Features

- 🔔 **Windows toast notifications** with fallback popup dialog
- 🎛️ **Per-tool settings** — configure Codex and Claude Code independently
- ⚡ **Tab-based interactive UI** — switch between Default / Codex / Claude Code tabs
- 📋 **Content modes** — summary, full message, or minimal "complete" text
- 💾 **Persistent settings** — preferences survive restarts
- 🔌 **One-click install** — auto-configures hooks for both tools

## Quick Start

### Option A: Download Release

1. Download the latest release from [Releases](https://github.com/anthropics/cc-notify/releases)
2. Extract and double-click `install.cmd`
3. Done — notifications are enabled for both Codex CLI and Claude Code

### Option B: Build from Source

```powershell
git clone https://github.com/anthropics/cc-notify.git
cd cc-notify
go build -o dist/cc-notify.exe ./cmd/cc-notify
./dist/cc-notify.exe install
```

## Usage

```
cc-notify                              interactive settings
cc-notify install [codex|claude]       register hooks (both if omitted)
cc-notify uninstall [codex|claude]     remove hooks (both if omitted)
cc-notify notify <json>                handle Codex event payload
cc-notify notify --claude              handle Claude Code hook (stdin)
cc-notify notify --file <path>         read payload from file
cc-notify notify --b64 <base64>        base64 encoded payload
cc-notify test-notify [title] [body]   send test notification
cc-notify test-toast [title] [body]    test toast mode
cc-notify help                         show this help
```

## Interactive UI

Run `cc-notify` with no arguments to open the interactive control center:

![img.png](asset/img.png)

**Tabs:**
- **Default** — Global settings inherited by all tools
- **Codex** — Override mode/content/enabled for Codex CLI only
- **Claude Code** — Override mode/content/enabled for Claude Code only

Each tool tab can be set to `inherit` (use Default) or have its own custom mode and content.

## Notification Modes

| Mode | Description |
|------|-------------|
| `auto` | Try toast first, fall back to popup dialog |
| `toast` | Windows system notification (requires Start Menu shortcut) |
| `popup` | Always use popup dialog |
**toast:**
![toast.png](asset/toast.png)


## Content Modes

| Mode | Description |
|------|-------------|
| `summary` | Short summary of what happened |
| `full` | Full assistant message |
| `complete` | Minimal "complete" text |

## How It Works

### Codex CLI
Registers a `notify` command in `~/.codex/config.toml`. When Codex finishes a task, it calls `cc-notify notify <json>` with the event payload.

### Claude Code
Registers a `Stop` hook in `~/.claude/settings.json`. When Claude Code finishes, it pipes the hook payload to `cc-notify notify --claude` via stdin.

## Configuration

Settings are stored in `%LOCALAPPDATA%\cc-notify\settings.json`:

```json
{
  "enabled": true,
  "persist": true,
  "mode": "auto",
  "content": "summary",
  "include_dir": true,
  "include_model": false,
  "include_event": false,
  "toast_app_id": "cc-notify.desktop",
  "codex_mode": "",
  "codex_content": "",
  "claude_mode": "popup",
  "claude_content": "full"
}
```

Per-tool fields (`codex_mode`, `claude_mode`, etc.) override the global defaults when set. Empty string means inherit from Default.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CC_NOTIFY_MODE` | Override notification mode (`auto`/`toast`/`popup`) |
| `CC_NOTIFY_TOAST_APP_ID` | Override toast Application User Model ID |
| `CC_NOTIFY_NO_PAUSE` | Set to `1` to disable "Press Enter to exit" on Windows |

## Uninstall

```powershell
cc-notify uninstall
```

Or double-click `uninstall.cmd` from the release folder.

## License

[MIT](LICENSE)

