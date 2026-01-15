package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// カラー定義（lazydocker風）
var (
	cyan       = lipgloss.Color("86")
	darkCyan   = lipgloss.Color("30")
	white      = lipgloss.Color("255")
	gray       = lipgloss.Color("240")
	darkGray   = lipgloss.Color("236")
	green      = lipgloss.Color("78")
	yellow     = lipgloss.Color("220")
	red        = lipgloss.Color("196")

	// ボーダースタイル
	borderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(gray)

	activeBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(cyan)

	// タイトルスタイル
	titleStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	// 選択行スタイル
	selectedStyle = lipgloss.NewStyle().
			Background(darkCyan).
			Foreground(white)

	// ステータス色
	statusRunning = lipgloss.NewStyle().Foreground(green)
	statusIdle    = lipgloss.NewStyle().Foreground(gray)
	statusWaiting = lipgloss.NewStyle().Foreground(yellow)
	statusDone    = lipgloss.NewStyle().Foreground(cyan)

	// ヘルプスタイル
	helpKeyStyle  = lipgloss.NewStyle().Foreground(cyan)
	helpDescStyle = lipgloss.NewStyle().Foreground(gray)
)

type session struct {
	TabID    int
	WindowID int
	Title    string
	AI       string
	Status   string
	Lines    []string
	Updated  time.Time
	Cwd      string
}

type model struct {
	width      int
	height     int
	selected   int
	sessions   []session
	err        error
	pollEvery  time.Duration
	prefixes   []string
	maxLines   int
	lastUpdate time.Time
	// リネームモード
	renaming    bool
	renameInput []rune // runeスライスで日本語対応
}

type tickMsg time.Time

type kittyOSWindow struct {
	Tabs []kittyTab `json:"tabs"`
}

type kittyTab struct {
	ID      int           `json:"id"`
	Title   string        `json:"title"`
	Windows []kittyWindow `json:"windows"`
}

type kittyWindow struct {
	ID                  int                 `json:"id"`
	Title               string              `json:"title"`
	Cwd                 string              `json:"cwd"`
	ForegroundProcesses []foregroundProcess `json:"foreground_processes"`
}

type foregroundProcess struct {
	Pid     int      `json:"pid"`
	Cwd     string   `json:"cwd"`
	Cmdline []string `json:"cmdline"`
}

var debugMode bool

func main() {
	pollEvery := flag.Duration("poll", 1*time.Second, "poll interval")
	prefixes := flag.String("prefixes", "codex,claude,gemini", "comma-separated process names to detect")
	maxLines := flag.Int("max-lines", 200, "max lines to keep per session")
	debug := flag.Bool("debug", false, "dump debug info and exit")
	noAltScreen := flag.Bool("no-alt-screen", false, "run without alt screen (for debugging)")
	kittySocket := flag.String("kitty-socket", "", "kitty socket path (e.g., unix:/tmp/mykitty)")
	flag.Parse()

	debugMode = *debug

	// Set kitty socket path from flag, environment, or auto-detect
	kittySocketPath = *kittySocket
	if kittySocketPath == "" {
		kittySocketPath = os.Getenv("KITTY_LISTEN_ON")
	}
	if kittySocketPath == "" {
		// Try to auto-detect socket path from KITTY_PID
		if pid := os.Getenv("KITTY_PID"); pid != "" {
			socketPath := fmt.Sprintf("/tmp/kitty-%s", pid)
			if _, err := os.Stat(socketPath); err == nil {
				kittySocketPath = "unix:" + socketPath
			}
		}
	}

	if debugMode {
		runDebug(parsePrefixes(*prefixes), *maxLines)
		return
	}

	// Enable debug logging to file
	var err error
	debugLog, err = os.Create("/tmp/lazyccg-tui.log")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create debug log:", err)
	}
	if debugLog != nil {
		defer debugLog.Close()
	}

	m := model{
		pollEvery: *pollEvery,
		prefixes:  parsePrefixes(*prefixes),
		maxLines:  *maxLines,
	}

	var p *tea.Program
	if *noAltScreen {
		p = tea.NewProgram(m)
	} else {
		p = tea.NewProgram(m, tea.WithAltScreen())
	}
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDebug(prefixes []string, maxLines int) {
	fmt.Println("=== lazyccg debug ===")
	fmt.Println("prefixes:", prefixes)
	fmt.Println()

	// Get raw kitty output
	cmd := exec.Command("kitty", "@", "ls")
	rawOut, err := cmd.Output()
	if err != nil {
		fmt.Println("kitty @ ls error:", err)
		return
	}

	// Parse and show structure
	var osWindows []kittyOSWindow
	if err := json.Unmarshal(rawOut, &osWindows); err != nil {
		fmt.Println("JSON parse error:", err)
		return
	}

	fmt.Println("=== parsed windows ===")
	for i, ow := range osWindows {
		fmt.Printf("OS Window %d:\n", i)
		for j, tab := range ow.Tabs {
			fmt.Printf("  Tab %d (id=%d, title=%q):\n", j, tab.ID, tab.Title)
			for k, win := range tab.Windows {
				fmt.Printf("    Window %d (id=%d, title=%q):\n", k, win.ID, win.Title)
				fmt.Printf("      Cwd: %s\n", win.Cwd)
				fmt.Printf("      ForegroundProcesses: %d\n", len(win.ForegroundProcesses))
				for l, proc := range win.ForegroundProcesses {
					fmt.Printf("        [%d] pid=%d cmdline=%v\n", l, proc.Pid, proc.Cmdline)
				}
				// Check if this window matches
				ai, ok := extractAI(win, prefixes)
				fmt.Printf("      extractAI result: ai=%q ok=%v\n", ai, ok)
			}
		}
	}
	fmt.Println()

	// Load sessions
	sessions, err := loadSessions(prefixes, maxLines)
	if err != nil {
		fmt.Println("loadSessions error:", err)
	} else {
		fmt.Printf("=== detected sessions: %d ===\n", len(sessions))
		for i, s := range sessions {
			fmt.Printf("  [%d] AI=%s Title=%q Status=%s WindowID=%d\n", i, s.AI, s.Title, s.Status, s.WindowID)
		}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tick(m.pollEvery))
}

type renameResultMsg struct {
	err error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// リネームモード中の処理
		if m.renaming {
			switch msg.Type {
			case tea.KeyEnter:
				if len(m.sessions) > 0 && m.selected >= 0 && m.selected < len(m.sessions) {
					windowID := m.sessions[m.selected].WindowID
					newTitle := string(m.renameInput)
					m.renaming = false
					m.renameInput = nil
					return m, renameCmd(windowID, newTitle)
				}
				m.renaming = false
				m.renameInput = nil
			case tea.KeyEsc:
				m.renaming = false
				m.renameInput = nil
			case tea.KeyBackspace:
				if len(m.renameInput) > 0 {
					m.renameInput = m.renameInput[:len(m.renameInput)-1]
				}
			case tea.KeySpace:
				m.renameInput = append(m.renameInput, ' ')
			case tea.KeyRunes:
				m.renameInput = append(m.renameInput, msg.Runes...)
			}
			return m, nil
		}

		// 通常モード
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if len(m.sessions) > 0 && m.selected >= 0 && m.selected < len(m.sessions) {
				return m, focusCmd(m.sessions[m.selected].WindowID)
			}
		case "r":
			if len(m.sessions) > 0 && m.selected >= 0 && m.selected < len(m.sessions) {
				m.renaming = true
				m.renameInput = []rune(m.sessions[m.selected].Title)
			}
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.sessions)-1 {
				m.selected++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), tick(m.pollEvery))
	case []session:
		m.sessions = msg
		if m.selected >= len(m.sessions) {
			m.selected = len(m.sessions) - 1
			if m.selected < 0 {
				m.selected = 0
			}
		}
		m.lastUpdate = time.Now()
	case renameResultMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.lastUpdate = time.Now()
	case error:
		m.err = msg
		m.lastUpdate = time.Now()
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// レイアウト計算
	helpHeight := 1
	contentHeight := m.height - helpHeight
	if contentHeight < 3 {
		contentHeight = 3
	}

	leftWidth := 40
	if m.width < 80 {
		leftWidth = m.width / 2
	}
	rightWidth := m.width - leftWidth

	// パネル内部の高さ（ボーダー分を引く）
	innerHeight := contentHeight - 2

	left := m.renderSessionsPanel(leftWidth, innerHeight)
	right := m.renderOutputPanel(rightWidth, innerHeight)

	content := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := m.renderHelp(m.width)

	return content + "\n" + help
}

func (m model) renderSessionsPanel(width, innerHeight int) string {
	// タイトルを作成
	title := titleStyle.Render(" Sessions ")

	// 同じタブIDが複数あるかチェック
	tabCount := make(map[int]int)
	for _, s := range m.sessions {
		tabCount[s.TabID]++
	}

	// コンテンツを作成
	var lines []string
	if len(m.sessions) == 0 {
		lines = append(lines, helpDescStyle.Render("  (no sessions)"))
	} else {
		for i, s := range m.sessions {
			name := s.Title
			if name == "" {
				name = fmt.Sprintf("tab-%d", s.TabID)
			}

			// 同じタブに複数ウィンドウがある場合はCWDのbasenameを追加
			if tabCount[s.TabID] > 1 && s.Cwd != "" {
				cwdBase := filepath.Base(s.Cwd)
				name = fmt.Sprintf("%s/%s", name, cwdBase)
			}

			// AI名とステータスをフォーマット
			ai := strings.ToUpper(s.AI)
			status := m.formatStatus(s.Status)

			// 行を構築: タブ名 (AI)  STATUS
			line := fmt.Sprintf(" %s (%s)  %s", name, ai, status)

			// 選択されている場合はハイライト
			if i == m.selected {
				// 幅に合わせてパディング
				paddedLine := line
				if len(line) < width-4 {
					paddedLine = line + strings.Repeat(" ", width-4-len(line))
				}
				line = selectedStyle.Render(paddedLine)
			}
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")

	// 高さを調整
	lineCount := len(lines)
	if lineCount < innerHeight {
		content += strings.Repeat("\n", innerHeight-lineCount)
	}

	// ボーダー付きパネルを作成
	panel := activeBorderStyle.
		Width(width - 2).
		Height(innerHeight).
		Render(content)

	// タイトルをボーダーに埋め込む
	return embedTitle(panel, title)
}

func (m model) renderOutputPanel(width, innerHeight int) string {
	title := titleStyle.Render(" Output ")

	var content string
	if len(m.sessions) == 0 || m.selected >= len(m.sessions) {
		content = helpDescStyle.Render("  (no output)")
	} else {
		logs := m.sessions[m.selected].Lines
		if len(logs) == 0 {
			content = helpDescStyle.Render("  (empty)")
		} else {
			// 表示する行数を制限
			displayLines := logs
			if len(displayLines) > innerHeight {
				displayLines = displayLines[len(displayLines)-innerHeight:]
			}
			content = strings.Join(displayLines, "\n")
		}
	}

	panel := borderStyle.
		Width(width - 2).
		Height(innerHeight).
		Render(content)

	return embedTitle(panel, title)
}

func embedTitle(panel, title string) string {
	lines := strings.Split(panel, "\n")
	if len(lines) == 0 {
		return panel
	}
	// 最初の行にタイトルを埋め込む
	firstLine := lines[0]
	runes := []rune(firstLine)
	titleRunes := []rune(title)

	// タイトルを埋め込む位置（左から2文字目以降）
	insertPos := 2
	if len(runes) > insertPos+len(titleRunes) {
		for i, r := range titleRunes {
			runes[insertPos+i] = r
		}
		lines[0] = string(runes)
	}
	return strings.Join(lines, "\n")
}

func (m model) formatStatus(status string) string {
	padded := fmt.Sprintf("%-7s", status)
	switch status {
	case "RUNNING":
		return statusRunning.Render(padded)
	case "IDLE":
		return statusIdle.Render(padded)
	case "WAITING":
		return statusWaiting.Render(padded)
	case "DONE":
		return statusDone.Render(padded)
	default:
		return padded
	}
}

func (m model) renderHelp(width int) string {
	if m.renaming {
		// リネームモード
		input := string(m.renameInput)
		prompt := helpKeyStyle.Render("Rename: ") + input + "█"
		hint := helpDescStyle.Render(" (enter: confirm, esc: cancel)")
		return prompt + hint
	}

	// 通常モードのヘルプ
	items := []string{
		helpKeyStyle.Render("↑↓/jk") + helpDescStyle.Render(": navigate"),
		helpKeyStyle.Render("enter") + helpDescStyle.Render(": focus"),
		helpKeyStyle.Render("r") + helpDescStyle.Render(": rename"),
		helpKeyStyle.Render("q") + helpDescStyle.Render(": quit"),
	}

	help := strings.Join(items, "  ")

	// 右側に更新時刻
	if !m.lastUpdate.IsZero() {
		updated := helpDescStyle.Render(m.lastUpdate.Format("15:04:05"))
		padding := width - lipgloss.Width(help) - lipgloss.Width(updated) - 2
		if padding > 0 {
			help += strings.Repeat(" ", padding) + updated
		}
	}

	return help
}

func (m model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := loadSessions(m.prefixes, m.maxLines)
		if err != nil {
			return err
		}
		return sessions
	}
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func focusCmd(windowID int) tea.Cmd {
	return func() tea.Msg {
		if windowID == 0 {
			return nil
		}
		args := []string{"@"}
		if kittySocketPath != "" {
			args = append(args, "--to", kittySocketPath)
		}
		args = append(args, "focus-window", "--match", fmt.Sprintf("id:%d", windowID))

		if err := exec.Command("kitty", args...).Run(); err != nil {
			return err
		}
		return nil
	}
}

func renameCmd(windowID int, title string) tea.Cmd {
	return func() tea.Msg {
		if windowID == 0 {
			return renameResultMsg{err: nil}
		}
		args := []string{"@"}
		if kittySocketPath != "" {
			args = append(args, "--to", kittySocketPath)
		}
		args = append(args, "set-window-title", "--match", fmt.Sprintf("id:%d", windowID), title)

		if err := exec.Command("kitty", args...).Run(); err != nil {
			return renameResultMsg{err: err}
		}
		return renameResultMsg{err: nil}
	}
}

var debugLog *os.File

func loadSessions(prefixes []string, maxLines int) ([]session, error) {
	if debugLog != nil {
		fmt.Fprintf(debugLog, "[%s] loadSessions called, prefixes=%v\n", time.Now().Format("15:04:05"), prefixes)
	}

	osWindows, err := kittyList()
	if err != nil {
		if debugLog != nil {
			fmt.Fprintf(debugLog, "[%s] kittyList error: %v\n", time.Now().Format("15:04:05"), err)
		}
		return nil, err
	}

	if debugLog != nil {
		fmt.Fprintf(debugLog, "[%s] kittyList returned %d OS windows\n", time.Now().Format("15:04:05"), len(osWindows))
	}

	var sessions []session
	for _, ow := range osWindows {
		for _, tab := range ow.Tabs {
			for _, win := range tab.Windows {
				if debugLog != nil {
					fmt.Fprintf(debugLog, "[%s] checking tab=%q win=%d procs=%d\n",
						time.Now().Format("15:04:05"), tab.Title, win.ID, len(win.ForegroundProcesses))
				}
				ai, ok := extractAI(win, prefixes)
				if !ok {
					continue
				}
				text, _ := kittyGetText(win.ID)
				lines := normalizeLines(text, maxLines)
				status := inferStatus(lines)
				// Use window title if available, otherwise tab title, otherwise cwd
				title := win.Title
				if title == "" {
					title = tab.Title
				}
				if title == "" {
					title = win.Cwd
				}
				sessions = append(sessions, session{
					TabID:    tab.ID,
					WindowID: win.ID,
					Title:    title,
					AI:       ai,
					Status:   status,
					Lines:    lines,
					Updated:  time.Now(),
					Cwd:      win.Cwd,
				})
			}
		}
	}

	if debugLog != nil {
		fmt.Fprintf(debugLog, "[%s] returning %d sessions\n", time.Now().Format("15:04:05"), len(sessions))
	}

	return sortSessions(sessions), nil
}

func sortSessions(sessions []session) []session {
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].AI == sessions[j].AI {
			return sessions[i].Title < sessions[j].Title
		}
		return sessions[i].AI < sessions[j].AI
	})
	return sessions
}

func extractAI(win kittyWindow, prefixes []string) (string, bool) {
	for _, proc := range win.ForegroundProcesses {
		if len(proc.Cmdline) == 0 {
			continue
		}
		// Check all cmdline elements for AI tool names
		for _, arg := range proc.Cmdline {
			argLower := strings.ToLower(arg)
			for _, p := range prefixes {
				// Check basename (e.g., /usr/bin/claude -> claude)
				base := arg
				if idx := strings.LastIndex(arg, "/"); idx >= 0 {
					base = arg[idx+1:]
				}
				baseLower := strings.ToLower(base)
				if baseLower == p {
					return p, true
				}
				// Check if path contains the prefix as a component
				// e.g., /path/to/@openai/codex/bin/codex
				if strings.Contains(argLower, "/"+p+"/") || strings.Contains(argLower, "/"+p) {
					return p, true
				}
			}
		}
	}
	return "", false
}

func parsePrefixes(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var kittySocketPath string

func kittyList() ([]kittyOSWindow, error) {
	args := []string{"@"}
	if kittySocketPath != "" {
		args = append(args, "--to", kittySocketPath)
	}
	args = append(args, "ls")

	cmd := exec.Command("kitty", args...)
	out, err := cmd.Output()

	if debugLog != nil {
		fmt.Fprintf(debugLog, "[kittyList] socket=%q err=%v out_len=%d\n",
			kittySocketPath, err, len(out))
	}

	if err != nil {
		return nil, fmt.Errorf("kitty @ ls: %w", err)
	}
	var osWindows []kittyOSWindow
	if err := json.Unmarshal(out, &osWindows); err != nil {
		return nil, fmt.Errorf("json parse: %w", err)
	}
	return osWindows, nil
}

func kittyGetText(windowID int) (string, error) {
	args := []string{"@"}
	if kittySocketPath != "" {
		args = append(args, "--to", kittySocketPath)
	}
	args = append(args, "get-text", "--match", fmt.Sprintf("id:%d", windowID))

	cmd := exec.Command("kitty", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func normalizeLines(text string, maxLines int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\t ")
		if line == "" {
			continue
		}
		trimmed = append(trimmed, line)
	}
	if maxLines > 0 && len(trimmed) > maxLines {
		trimmed = trimmed[len(trimmed)-maxLines:]
	}
	return trimmed
}

func inferStatus(lines []string) string {
	if len(lines) == 0 {
		return "IDLE"
	}

	// 最終行をチェック（プロンプト待ち検出）
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	lastLineLower := strings.ToLower(lastLine)

	// プロンプト待ちパターン（最終行が入力待ちの場合）
	if strings.HasPrefix(lastLine, "> ") || // Codex, Claude等
		strings.HasPrefix(lastLine, "$ ") || // シェルプロンプト
		strings.HasPrefix(lastLine, "% ") || // zshプロンプト
		strings.HasSuffix(lastLine, "> ") ||
		strings.HasSuffix(lastLine, "$ ") ||
		strings.HasSuffix(lastLine, "% ") ||
		strings.Contains(lastLineLower, "context left") || // Codex特有
		strings.Contains(lastLineLower, "? for shortcuts") { // Codex特有
		return "IDLE"
	}

	lookback := 20
	start := 0
	if len(lines) > lookback {
		start = len(lines) - lookback
	}
	chunk := strings.ToLower(strings.Join(lines[start:], " "))
	if strings.Contains(chunk, "waiting") ||
		strings.Contains(chunk, "approval") ||
		strings.Contains(chunk, "confirm") ||
		strings.Contains(chunk, "press enter") {
		return "WAITING"
	}
	if strings.Contains(chunk, "done") ||
		strings.Contains(chunk, "finished") ||
		strings.Contains(chunk, "complete") {
		return "DONE"
	}
	return "RUNNING"
}
