# lazyccg

A lightweight TUI dashboard for monitoring Claude Code / Codex / Gemini sessions in kitty terminal.

## Features

- View all AI sessions at a glance
- Auto-detect session status (RUNNING / IDLE / WAITING / DONE)
- lazydocker-style split pane UI
- Rename sessions with Japanese input support
- Quick focus to any session

## Supported AI Tools

- Claude Code
- OpenAI Codex
- Gemini CLI

## Installation

### Homebrew (macOS)

```bash
brew tap atani/tap
brew install lazyccg
```

### Go Install

```bash
go install github.com/atani/lazyccg/cmd/lazyccg@latest
```

### Build from Source

```bash
git clone https://github.com/atani/lazyccg.git
cd lazyccg
go build ./cmd/lazyccg
```

## Prerequisites

- [kitty](https://sw.kovidgoyal.net/kitty/) terminal with remote control enabled
- Add to your `kitty.conf`:
  ```
  allow_remote_control yes
  listen_on unix:/tmp/kitty
  ```

## Usage

```bash
lazyccg
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-poll` | Refresh interval | `1s` |
| `-prefixes` | AI tool prefixes to detect | `codex,claude,gemini` |
| `-max-lines` | Max lines to keep per session | `200` |
| `-debug` | Dump debug info and exit | `false` |
| `-no-alt-screen` | Run without alt screen (for debugging) | `false` |
| `-kitty-socket` | Kitty socket path (e.g., `unix:/tmp/mykitty`) | auto-detect |

### Keybindings

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Focus selected session / Select filter |
| `r` | Rename session |
| `Tab` | Switch to Status panel (filter) |
| `Esc` | Clear filter / Back to Sessions |
| `q` | Quit |

## Screenshot

```
╭ Sessions ─────────────────────────────╮╭ Output ──────────────────────────────╮
│ CLAUDE  IDLE     * Claude Code        ││ > Hello! How can I help you?         │
│ CLAUDE  RUNNING  project-x            ││                                      │
│ CODEX   IDLE     codex                ││                                      │
╰───────────────────────────────────────╯╰──────────────────────────────────────╯
↑↓/jk: navigate  enter: focus  r: rename  q: quit                      12:34:56
```

## License

MIT
