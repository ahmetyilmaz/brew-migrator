package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── Screen state ──────────────────────────────────────────────────────────

type screen int

const (
	screenMenu screen = iota
	screenScanning
	screenSelection
	screenInstalling
	screenSummary
	screenHistory
)

// ── Messages ──────────────────────────────────────────────────────────────

type (
	scanProgressMsg struct {
		idx  int
		info AppInfo
	}
	scanDoneMsg        struct{}
	installProgressMsg struct {
		idx     int
		name    string
		success bool
		err     string
	}
	installDoneMsg    struct{}
	historyLoadedMsg  struct {
		entries map[string]HistoryEntry
	}
)

// ── Key bindings ──────────────────────────────────────────────────────────

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Back     key.Binding
	Quit     key.Binding
	Space    key.Binding
	Enter    key.Binding
	Help     key.Binding
}

var keys = keyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "go back")),
	Quit:   key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("ctrl+c/q", "quit")),
	Space:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// ── Menu items ────────────────────────────────────────────────────────────

type menuItem struct {
	title       string
	description string
	action      func(*model) tea.Cmd
}

func buildMenu(m *model) []menuItem {
	planSize := len(m.selectedApps)
	histSize := len(m.historyEntries)

	return []menuItem{
		{
			title: "🔍  Scan & Plan",
			description: "Discover apps in /Applications and match to Homebrew Casks",
			action: func(m *model) tea.Cmd { return m.startScan() },
		},
		{
			title: "🚀  Install Selected Plan",
			description: func() string {
				if planSize > 0 {
					return fmt.Sprintf("(%d apps ready to install)", planSize)
				}
				return "(no plan yet)"
			}(),
			action: func(m *model) tea.Cmd {
				if len(m.selectedApps) == 0 {
					return nil
				}
				return m.startInstall()
			},
		},
		{
			title: "👀  View History",
			description: fmt.Sprintf("(%d entries)", histSize),
			action: func(m *model) tea.Cmd { return m.loadHistory() },
		},
		{
			title: "🗑  Clear History",
			description: func() string {
				if histSize == 0 {
					return "(nothing to clear)"
				}
				return "Remove all history entries"
			}(),
			action: func(m *model) tea.Cmd {
				saveHistory(make(map[string]HistoryEntry))
				m.historyEntries = loadHistory()
				return nil
			},
		},
		{
			title: "🚪  Quit",
			description: "Exit Brew Migrator",
			action: func(m *model) tea.Cmd { return tea.Quit },
		},
	}
}

// ── Model ─────────────────────────────────────────────────────────────────

type model struct {
	// State machine
	state   screen
	width   int
	height  int
	ready   bool
	quitting bool

	// Menu
	menuCursor int
	menuItems  []menuItem

	// Scanning
	scanApps    []AppInfo
	scanCurrent int
	scanTotal   int
	scanDone    bool

	// Selection
	selCursor   int
	selApps     []*AppInfo
	selectedApps []*AppInfo
	selViewport viewport.Model

	// Installing
	installApps     []*AppInfo
	installProgress progress.Model
	installCurrent  int
	installTotal    int
	installResults  []installResult
	installDone     bool

	// Summary
	summarySuccess int
	summaryFail    int
	summaryApps    []string

	// History
	historyEntries map[string]HistoryEntry
	historyTable   table.Model

	// Shared
	spinner spinner.Model
	help    help.Model
	err     error
}

type installResult struct {
	name    string
	success bool
	err     string
}

func newModel() *model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrand))
	s.Spinner = spinner.Dot

	p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(40))
	h := help.New()
	h.Width = 80

	m := &model{
		state:          screenMenu,
		menuCursor:     0,
		spinner:        s,
		help:           h,
		installProgress: p,
		selViewport:    viewport.New(80, 10),
		historyEntries: loadHistory(),
	}
	m.menuItems = buildMenu(m)
	return m
}

// ── Init ──────────────────────────────────────────────────────────────────

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tea.EnterAltScreen)
}

// ── Update ────────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width
		m.selViewport.Width = msg.Width - 4
		m.selViewport.Height = msg.Height - 12

		// Rebuild width-dependent views
		p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(msg.Width-20))
		m.installProgress = p
		return m, nil

	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}
		switch m.state {
		case screenMenu:
			return m.updateMenu(msg)
		case screenScanning:
			return m.updateScanning(msg)
		case screenSelection:
			return m.updateSelection(msg)
		case screenInstalling:
			return m.updateInstalling(msg)
		case screenSummary:
			return m.updateSummary(msg)
		case screenHistory:
			return m.updateHistory(msg)
		}

	case scanProgressMsg:
		return m.handleScanProgress(msg)
	case scanDoneMsg:
		return m.handleScanDone()
	case installProgressMsg:
		return m.handleInstallProgress(msg)
	case installDoneMsg:
		return m.handleInstallDone()
	case historyLoadedMsg:
		return m.handleHistoryLoaded(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		pm, cmd := m.installProgress.Update(msg)
		m.installProgress = pm.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────

func (m *model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	switch m.state {
	case screenMenu:
		return m.renderMenu()
	case screenScanning:
		return m.renderScanning()
	case screenSelection:
		return m.renderSelection()
	case screenInstalling:
		return m.renderInstalling()
	case screenSummary:
		return m.renderSummary()
	case screenHistory:
		return m.renderHistory()
	}
	return "unknown state"
}

// ═══════════════════════════════════════════════════════════════════════════
// MENU SCREEN
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.menuItems = buildMenu(m)
	switch {
	case key.Matches(msg, keys.Up):
		m.menuCursor--
		if m.menuCursor < 0 {
			m.menuCursor = len(m.menuItems) - 1
		}
	case key.Matches(msg, keys.Down):
		m.menuCursor++
		if m.menuCursor >= len(m.menuItems) {
			m.menuCursor = 0
		}
	case key.Matches(msg, keys.Select), key.Matches(msg, keys.Enter):
		if m.menuCursor >= 0 && m.menuCursor < len(m.menuItems) {
			cmd := m.menuItems[m.menuCursor].action(m)
			if cmd != nil {
				return m, cmd
			}
		}
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) renderMenu() string {
	var b strings.Builder

	// Refresh menu items so plan/history counts are live
	m.menuItems = buildMenu(m)

	// ── Title block ──
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorBrand)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorSurface)).
		Padding(0, 2).
		Render("  🍺  Brew Migrator  ")

	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).Render(title))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).
		Foreground(lipgloss.Color(colorTextDim)).
		Render("  scan  →  match  →  select  →  install"))
	b.WriteString("\n\n")

	// ── Menu items ──
	for i, item := range m.menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = CursorStyle.Render("▸")
		}

		titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)).Padding(0, 1)
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Italic(true).Padding(0, 1)

		if i == m.menuCursor {
			titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorBrand)).
				Bold(true).
				Background(lipgloss.Color(colorBgLight)).
				Padding(0, 1)
		}

		line := fmt.Sprintf("    %s %s\n         %s",
			cursor,
			titleStyle.Render(item.title),
			descStyle.Render(item.description))
		b.WriteString(line)
		b.WriteString("\n")
	}

	// ── Help bar ──
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorTextDim)).
		Padding(0, 2).
		Render("  ↑/k ↓/j navigate  •  enter select  •  q quit  •  ? help"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// SCANNING SCREEN — live table, updated per-app
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) startScan() tea.Cmd {
	m.state = screenScanning
	m.scanDone = false
	m.scanCurrent = 0

	// Phase 1: load installed casks into memory (fast, one brew call)
	loadInstalledCasks()

	appNames := scanApplications("/Applications")
	m.scanApps = make([]AppInfo, len(appNames))
	for i, name := range appNames {
		m.scanApps[i] = AppInfo{Name: name, Score: -1}
	}
	m.scanTotal = len(appNames)

	if m.scanTotal == 0 {
		return func() tea.Msg { return scanDoneMsg{} }
	}

	return m.scanNext()
}

func (m *model) scanNext() tea.Cmd {
	if m.scanCurrent >= m.scanTotal {
		return func() tea.Msg { return scanDoneMsg{} }
	}

	idx := m.scanCurrent
	name := m.scanApps[idx].Name

	return func() tea.Msg {
		time.Sleep(25 * time.Millisecond) // visual pacing
		candidates := getCaskCandidates(name)
		info := AppInfo{Name: name}

		if len(candidates) == 0 {
			info.Score = -1
			info.Skipped = true
		} else {
			info.Candidates = candidates
			info.BestCask = candidates[0].Name
			info.Score = candidates[0].Score
			info.Selected = info.Score > 90
		}
		return scanProgressMsg{idx: idx, info: info}
	}
}

func (m *model) handleScanProgress(msg scanProgressMsg) (tea.Model, tea.Cmd) {
	m.scanApps[msg.idx] = msg.info
	m.scanCurrent++
	return m, m.scanNext()
}

func (m *model) handleScanDone() (tea.Model, tea.Cmd) {
	m.scanDone = true
	m.state = screenSelection
	m.selCursor = 0
	m.selApps = make([]*AppInfo, len(m.scanApps))
	for i := range m.scanApps {
		m.selApps[i] = &m.scanApps[i]
	}
	return m, nil
}

func (m *model) updateScanning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		m.state = screenMenu
	}
	return m, nil
}

// scanInstalledCount returns how many scanned apps were fast-matched (score == 100)
// to an already-installed cask.
func scanInstalledCount(apps []AppInfo) int {
	n := 0
	for _, a := range apps {
		if a.Score == 100 {
			n++
		}
	}
	return n
}

func (m *model) renderScanning() string {
	var b strings.Builder

	// ── Header ──
	b.WriteString("\n")
	b.WriteString(AppTitleStyle.Render("  🔍  Scanning  " + m.spinner.View()))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("    %s\n\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).
			Render(fmt.Sprintf("matched %d / %d apps  •  %s  •  %d from installed casks",
				m.scanCurrent, m.scanTotal,
				m.spinner.View(),
				scanInstalledCount(m.scanApps)))))

	// ── Rows ──
	// Column headers
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBrand))
	b.WriteString(fmt.Sprintf("  %s  %-30s %-30s %-5s %s  %s\n",
		header.Render("#"),
		header.Render("App"),
		header.Render("Cask"),
		header.Render("Score"),
		header.Render("Quality"),
		header.Render("Sel")))
	b.WriteString(strings.Repeat("─", m.width-4) + "\n")

	// Body — show visible range around current
	visibleStart := max(0, m.scanCurrent-8)
	visibleEnd := min(len(m.scanApps), visibleStart+20)

	for i := visibleStart; i < visibleEnd; i++ {
		app := m.scanApps[i]
		isCurrent := i == m.scanCurrent-1 && !m.scanDone
		isDone := m.scanCurrent > i

		lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
		if isCurrent {
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrand)).Bold(true)
		}

		if !isDone && !isCurrent && !app.Skipped {
			// Pending
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim))
		}

		var row string
		if app.Skipped && isDone {
			row = fmt.Sprintf("  %-2s  %-30s %-30s",
				StatusSkipped, app.Name, lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("—"))
		} else if isDone && !app.Skipped {
			sel := "○"
			if app.Selected {
				sel = CheckboxCheckedStyle.Render("✓")
			}
			row = fmt.Sprintf("  %-2s  %-30s %-30s %-5d %s  %s",
				StatusSuccess,
				app.Name,
				app.BestCask,
				app.Score,
				QualityEmoji(app.Score),
				sel)
		} else if isCurrent {
			row = fmt.Sprintf("  %s  %-30s %-30s",
				m.spinner.View(), app.Name,
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("scanning…"))
		} else {
			continue // don't show pending rows
		}

		b.WriteString(lineStyle.Render(row) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorTextDim)).
		Render("  esc to cancel  •  q quit"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// SELECTION SCREEN — checkbox list with keyboard nav
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		m.state = screenMenu
		return m, nil
	case key.Matches(msg, keys.Up):
		if m.selCursor > 0 {
			m.selCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.selCursor < len(m.selApps)-1 {
			m.selCursor++
		}
	case key.Matches(msg, keys.Space):
		if m.selCursor >= 0 && m.selCursor < len(m.selApps) {
			app := m.selApps[m.selCursor]
			if !app.Skipped {
				app.Selected = !app.Selected
			}
		}
	case key.Matches(msg, keys.Select), key.Matches(msg, keys.Enter):
		m.selectedApps = make([]*AppInfo, 0)
		for _, app := range m.selApps {
			if app.Selected && !app.Skipped {
				m.selectedApps = append(m.selectedApps, app)
			}
		}
		if len(m.selectedApps) == 0 {
			m.state = screenMenu
			return m, nil
		}
		// Confirm with huh
		var confirmed bool
		err := huh.NewConfirm().
			Title("Install selected apps?").
			Description(fmt.Sprintf("%d app(s) will be installed via Homebrew", len(m.selectedApps))).
			Affirmative("Yes, install!").
			Negative("Review again").
			Value(&confirmed).Run()
		if err != nil || !confirmed {
			return m, nil
		}
		return m, m.startInstall()
	}
	return m, nil
}

func (m *model) renderSelection() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(AppTitleStyle.Render("  📦  Select Apps to Install"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).
		Render("    ↓↑ navigate  •  space toggle  •  enter confirm  •  esc back"))
	b.WriteString("\n\n")

	// Count
	selected := 0
	for _, app := range m.selApps {
		if app.Selected {
			selected++
		}
	}

	b.WriteString(fmt.Sprintf("  %s\n\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).
			Render(fmt.Sprintf("%d selected of %d", selected, len(m.selApps)))))

	// Visible range
	start := max(0, m.selCursor-10)
	end := min(len(m.selApps), start+22)

	for i := start; i < end; i++ {
		app := m.selApps[i]

		// Cursor
		cursor := "  "
		if i == m.selCursor {
			cursor = CursorStyle.Render("▸")
		}

		// Checkbox
		cb := CheckboxUncheckedStyle.Render("○")
		if app.Skipped {
			cb = CheckboxDisabledStyle.Render("⛌")
		} else if app.Selected {
			cb = CheckboxCheckedStyle.Render("●")
		}

		// Build line
		var line string
		if app.Skipped {
			line = fmt.Sprintf("%s %s %s  %s",
				cursor, cb, app.Name,
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).
					Render("(no cask found)"))
		} else {
			quality := QualityEmoji(app.Score)
			scoreStr := lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Render(fmt.Sprintf("[%d]", app.Score))
			line = fmt.Sprintf("%s %s  %s  %s %s %s",
				cursor, cb, app.Name, quality, app.BestCask, scoreStr)
		}

		rowStyle := lipgloss.NewStyle().Padding(0, 1)
		if i == m.selCursor {
			rowStyle = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color(colorBgLight))
		}
		b.WriteString("  " + rowStyle.Render(line) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorTextDim)).
		Render("  ↓↑ move  •  space toggle  •  enter confirm  •  esc back  •  q quit"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// INSTALLING SCREEN — progress bar + results
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) startInstall() tea.Cmd {
	m.state = screenInstalling
	m.installApps = m.selectedApps
	m.installCurrent = 0
	m.installTotal = len(m.selectedApps)
	m.installResults = nil
	m.installDone = false

	// Reset progress bar
	p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(m.width-20))
	m.installProgress = p

	return m.installNext()
}

func (m *model) installNext() tea.Cmd {
	if m.installCurrent >= m.installTotal {
		return func() tea.Msg { return installDoneMsg{} }
	}

	app := m.installApps[m.installCurrent]
	return func() tea.Msg {
		success, errOut := installCask(app.BestCask)
		errMsg := ""
		if !success {
			errMsg = errOut
		}
		if success {
			updateHistory(app.Name, "migrated", app.BestCask)
		} else {
			updateHistory(app.Name, "failed", errMsg)
		}
		return installProgressMsg{
			idx:     m.installCurrent,
			name:    app.Name,
			success: success,
			err:     errMsg,
		}
	}
}

func (m *model) handleInstallProgress(msg installProgressMsg) (tea.Model, tea.Cmd) {
	m.installResults = append(m.installResults, installResult{
		name:    msg.name,
		success: msg.success,
		err:     msg.err,
	})
	m.installCurrent++
	pct := float64(m.installCurrent) / float64(m.installTotal)
	return m, tea.Batch(m.installProgress.SetPercent(pct), m.installNext())
}

func (m *model) handleInstallDone() (tea.Model, tea.Cmd) {
	m.installDone = true
	m.state = screenSummary
	m.summarySuccess = 0
	m.summaryFail = 0
	m.summaryApps = nil

	for _, r := range m.installResults {
		if r.success {
			m.summarySuccess++
			m.summaryApps = append(m.summaryApps,
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Render("✓  "+r.name))
		} else {
			m.summaryFail++
			m.summaryApps = append(m.summaryApps,
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorBad)).Render("✗  "+r.name+"  ("+r.err+")"))
		}
	}
	return m, nil
}

func (m *model) updateInstalling(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) renderInstalling() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(AppTitleStyle.Render("  🚀  Installing  " + m.spinner.View()))
	b.WriteString("\n\n")

	pct := float64(0)
	if m.installTotal > 0 {
		pct = float64(m.installCurrent) / float64(m.installTotal)
	}

	// Progress
	b.WriteString(fmt.Sprintf("  %s\n\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)).
			Render(fmt.Sprintf("Installing %d of %d", m.installCurrent, m.installTotal))))
	b.WriteString("  " + m.installProgress.ViewAs(pct) + "\n")
	b.WriteString(fmt.Sprintf("  %s\n\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true).
			Render(fmt.Sprintf("%d%%", int(pct*100)))))

	// Current
	if m.installCurrent < m.installTotal {
		app := m.installApps[m.installCurrent]
		b.WriteString(fmt.Sprintf("  %s  %s → %s\n\n",
			m.spinner.View(),
			lipgloss.NewStyle().Bold(true).Render(app.Name),
			app.BestCask))
	}

	// Recent results
	start := max(0, len(m.installResults)-8)
	for _, r := range m.installResults[start:] {
		if r.success {
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				StatusSuccess, r.name, lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Render("installed")))
		} else {
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				StatusFail, r.name, lipgloss.NewStyle().Foreground(lipgloss.Color(colorBad)).Render("failed")))
		}
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).
		Render("  Installing…  press q to interrupt"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// SUMMARY SCREEN
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Select), key.Matches(msg, keys.Enter), key.Matches(msg, keys.Back):
		m.state = screenMenu
	}
	return m, nil
}

func (m *model) renderSummary() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(AppTitleStyle.Render("  📊  Summary"))
	b.WriteString("\n\n")

	// Result box
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorSurface)).
		Padding(1, 2).
		Width(m.width - 8)

	inner := fmt.Sprintf("  %s\n\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBrand)).Render("Installation Complete!"))

	inner += fmt.Sprintf("  %s  %d succeeded\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true).Render("✓"),
		m.summarySuccess)

	if m.summaryFail > 0 {
		inner += fmt.Sprintf("  %s  %d failed\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color(colorBad)).Bold(true).Render("✗"),
			m.summaryFail)
	}

	if len(m.summaryApps) > 0 {
		inner += "\n"
		for _, line := range m.summaryApps {
			inner += fmt.Sprintf("    %s\n", line)
		}
	}

	b.WriteString(box.Render(inner))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorTextDim)).
		Render("  enter to return to menu  •  q quit"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// HISTORY SCREEN
// ═══════════════════════════════════════════════════════════════════════════

func (m *model) loadHistory() tea.Cmd {
	m.state = screenHistory
	return func() tea.Msg {
		return historyLoadedMsg{entries: loadHistory()}
	}
}

func (m *model) handleHistoryLoaded(msg historyLoadedMsg) (tea.Model, tea.Cmd) {
	m.historyEntries = msg.entries
	m.buildHistoryTable()
	return m, nil
}

func (m *model) buildHistoryTable() {
	columns := []table.Column{
		{Title: "App", Width: 30},
		{Title: "Status", Width: 10},
		{Title: "Detail", Width: 40},
		{Title: "When", Width: 25},
	}

	rows := make([]table.Row, 0, len(m.historyEntries))
	for _, entry := range m.historyEntries {
		rows = append(rows, table.Row{
			entry.App,
			entry.Status,
			entry.Detail,
			entry.Timestamp,
		})
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][3] > rows[j][3]
	})

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(min(len(rows)+1, 20)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true)
	s.Cell = s.Cell.Foreground(lipgloss.Color(colorText))
	s.Selected = s.Selected.Background(lipgloss.Color(colorBgLight))
	t.SetStyles(s)

	m.historyTable = t
}

func (m *model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Enter):
		m.state = screenMenu
	}
	return m, nil
}

func (m *model) renderHistory() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(AppTitleStyle.Render("  👀  Migration History"))
	b.WriteString(fmt.Sprintf("   %s\n\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).
			Render(fmt.Sprintf("(%d entries)", len(m.historyEntries)))))

	if len(m.historyEntries) > 0 {
		b.WriteString(m.historyTable.View())
	} else {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim)).
			PaddingLeft(2).
			Render("  No history yet. Run a scan and install some apps to see results here.\n"))
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorTextDim)).
		Render("  esc/enter back  •  q quit"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
