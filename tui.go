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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── Screen ──────────────────────────────────────────────────────

type screen int

const (
	screenMenu screen = iota
	screenScanning
	screenSelection
	screenInstalling
	screenSummary
	screenHistory
)

// ── Messages ────────────────────────────────────────────────────

type (
	scanProgressMsg struct {
		idx  int
		info AppInfo
	}
	scanDoneMsg         struct{}
	installProgressMsg  struct {
		idx     int
		name    string
		success bool
		err     string
	}
	installDoneMsg      struct{}
	historyLoadedMsg    struct {
		entries map[string]HistoryEntry
	}
	selectionDoneMsg    struct {
		selected []*AppInfo
	}
)

// ── Keys ────────────────────────────────────────────────────────

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Back   key.Binding
	Quit   key.Binding
	Space  key.Binding
	Help   key.Binding
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:  key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q", "quit")),
	Space: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// ── Menu ────────────────────────────────────────────────────────

type menuEntry struct {
	title       string
	desc        string
	action      func(*model) tea.Cmd
}

func buildMenu(m *model) []menuEntry {
	plan := len(m.selectedApps)
	hist := len(m.histEntries)
	return []menuEntry{
		{title: "🔍  Scan & Plan", desc: "Discover apps in /Applications and match to Homebrew Casks",
			action: func(m *model) tea.Cmd { return m.startScan() }},
		{title: "🚀  Install Selected Plan",
			desc: func() string {
				if plan > 0 { return fmt.Sprintf("(%d apps ready to install)", plan) }
				return "(no plan yet)"
			}(),
			action: func(m *model) tea.Cmd {
				if len(m.selectedApps) == 0 { return nil }
				return m.startInstall()
			}},
		{title: "👀  View History", desc: fmt.Sprintf("(%d entries)", hist),
			action: func(m *model) tea.Cmd { return m.loadHistory() }},
		{title: "🗑  Clear History",
			desc: func() string {
				if hist == 0 { return "(nothing to clear)" }
				return "Remove all history entries"
			}(),
			action: func(m *model) tea.Cmd {
				saveHistory(map[string]HistoryEntry{})
				m.histEntries = loadHistory()
				return nil
			}},
		{title: "🚪  Quit", desc: "Exit Brew Migrator",
			action: func(m *model) tea.Cmd { return tea.Quit }},
	}
}

// ── Model ───────────────────────────────────────────────────────

type model struct {
	state   screen
	width   int
	height  int
	ready   bool
	quitting bool

	// Menu
	menuCur int
	entries []menuEntry

	// Scan
	scanApps    []AppInfo
	scanCur     int
	scanTotal   int
	scanDone    bool

	// Selection  (from huh)
	selApps     []*AppInfo
	selectedApps []*AppInfo
	// Install
	installApps     []*AppInfo
	installProg     progress.Model
	installCur      int
	installTotal    int
	installResults  []installResult
	installDone     bool

	// Summary
	sumOK   int
	sumFail int
	sumRows []string

	// History
	histEntries map[string]HistoryEntry
	histTable   table.Model

	// Shared
	spinner spinner.Model
	help    help.Model
	err     error
}

type installResult struct {
	name    string
	ok      bool
	err     string
}

func NewModel() *model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(clrBrand)
	s.Spinner = spinner.Moon

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	h := help.New()
	h.Width = 80

	m := &model{
		state:       screenMenu,
		spinner:     s,
		help:        h,
		installProg: p,
		histEntries: loadHistory(),
	}
	m.entries = buildMenu(m)
	return m
}

// ── Init / Update / View ────────────────────────────────────────

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tea.EnterAltScreen)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width
		p := progress.New(progress.WithDefaultGradient(), progress.WithWidth(msg.Width-20), progress.WithoutPercentage())
		m.installProg = p
		return m, nil

	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}
		switch m.state {
		case screenMenu:       return m.updateMenu(msg)
		case screenScanning:   return m.updateScanning(msg)
		case screenSelection:  return m.updateSelection(msg)
		case screenInstalling: return m.updateInstalling(msg)
		case screenSummary:    return m.updateSummary(msg)
		case screenHistory:    return m.updateHistory(msg)
		}

	case scanProgressMsg:    return m.handleScanProgress(msg)
	case scanDoneMsg:        return m.handleScanDone()
	case installProgressMsg: return m.handleInstallProgress(msg)
	case installDoneMsg:     return m.handleInstallDone()
	case historyLoadedMsg:   return m.handleHistoryLoaded(msg)
	case selectionDoneMsg:   return m.handleSelectionDone(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		p, cmd := m.installProg.Update(msg)
		m.installProg = p.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m *model) View() string {
	if !m.ready {
		return "\n  " + m.spinner.View() + "  Loading…"
	}
	switch m.state {
	case screenMenu:       return m.renderMenu()
	case screenScanning:   return m.renderScanning()
	case screenSelection:  return m.renderSelection()
	case screenInstalling: return m.renderInstalling()
	case screenSummary:    return m.renderSummary()
	case screenHistory:    return m.renderHistory()
	}
	return "unknown state"
}

// ═════════════════════════════════════════════════════════════════
// MENU
// ═════════════════════════════════════════════════════════════════

func (m *model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.entries = buildMenu(m)
	switch {
	case key.Matches(msg, keys.Up):
		m.menuCur--
		if m.menuCur < 0 { m.menuCur = len(m.entries) - 1 }
	case key.Matches(msg, keys.Down):
		m.menuCur++
		if m.menuCur >= len(m.entries) { m.menuCur = 0 }
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Space):
		if m.menuCur >= 0 && m.menuCur < len(m.entries) {
			if cmd := m.entries[m.menuCur].action(m); cmd != nil {
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

	// ── Header ──
	title := TitleStyle.Render("  🍺  Brew Migrator  ")
	sub := SubtitleStyle.Render("    scan  →  match  →  select  →  install")
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(clrSurface).
			Padding(0, 1).Render(title),
		sub,
	))
	b.WriteString("\n\n")

	// ── Items ──
	for i, e := range m.entries {
		active := i == m.menuCur
		cur := "  "
		if active {
			cur = CursorArrow.Render("▸")
		}

		ti := MenuItemTitle.Render(e.title)
		de := MenuItemDesc.Render(e.desc)
		if active {
			ti = MenuItemTitleActive.Render(e.title)
		}

		block := fmt.Sprintf("  %s %s\n      %s", cur, ti, de)
		if active {
			block = ActiveCardStyle.Render(block)
		} else {
			block = CardStyle.Render(block)
		}
		b.WriteString(block)
		b.WriteString("\n")
	}

	// ── Footer ──
	b.WriteString(HelpFooter.Render("  ↑/k ↓/j navigate  •  enter select  •  q quit"))

	return DocStyle.Render(b.String())
}

// ═════════════════════════════════════════════════════════════════
// SCANNING
// ═════════════════════════════════════════════════════════════════

func (m *model) startScan() tea.Cmd {
	m.state = screenScanning
	m.scanDone = false
	m.scanCur = 0
	loadInstalledCasks()

	apps := scanApplications("/Applications")
	m.scanApps = make([]AppInfo, len(apps))
	for i, n := range apps {
		m.scanApps[i] = AppInfo{Name: n, Score: -1}
	}
	m.scanTotal = len(apps)
	if m.scanTotal == 0 {
		return func() tea.Msg { return scanDoneMsg{} }
	}
	return m.scanNext()
}

func (m *model) scanNext() tea.Cmd {
	if m.scanCur >= m.scanTotal {
		return func() tea.Msg { return scanDoneMsg{} }
	}
	idx := m.scanCur
	name := m.scanApps[idx].Name
	return func() tea.Msg {
		time.Sleep(20 * time.Millisecond)
		cands := getCaskCandidates(name)
		info := AppInfo{Name: name}
		if len(cands) == 0 {
			info.Score = -1
			info.Skipped = true
		} else {
			info.Candidates = cands
			info.BestCask = cands[0].Name
			info.Score = cands[0].Score
			info.Selected = cands[0].Score > 90
		}
		return scanProgressMsg{idx: idx, info: info}
	}
}

func (m *model) handleScanProgress(msg scanProgressMsg) (tea.Model, tea.Cmd) {
	m.scanApps[msg.idx] = msg.info
	m.scanCur++
	return m, m.scanNext()
}

func (m *model) handleScanDone() (tea.Model, tea.Cmd) {
	m.scanDone = true
	m.state = screenSelection
	m.selApps = make([]*AppInfo, len(m.scanApps))
	for i := range m.scanApps {
		m.selApps[i] = &m.scanApps[i]
	}
	// Immediately show the huh selection form
	return m, m.runSelectionForm()
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

func scanInstalledCount(apps []AppInfo) int {
	n := 0
	for _, a := range apps {
		if a.Score == 100 { n++ }
	}
	return n
}

func (m *model) renderScanning() string {
	var b strings.Builder

	// ── Header ──
	hdr := TitleStyle.Render(fmt.Sprintf("  🔍  Scanning %s", m.spinner.View()))
	b.WriteString("\n")
	b.WriteString(hdr)
	b.WriteString("\n")

	info := fmt.Sprintf("  matched %d / %d apps  •  %d from installed casks",
		m.scanCur, m.scanTotal, scanInstalledCount(m.scanApps[:m.scanCur]))
	b.WriteString(SubtitleStyle.Render(info))
	b.WriteString("\n\n")

	// ── Table ──
	// Column headers
	b.WriteString(fmt.Sprintf("  %s  %-28s %-26s %s  %s\n",
		TableHeader.Render("#"),
		TableHeader.Render("App"),
		TableHeader.Render("Cask"),
		TableHeader.Render("Q"),
		TableHeader.Render("Sel")))
	b.WriteString("  " + strings.Repeat("─", m.width-10) + "\n")

	showStart := max(0, m.scanCur-12)
	showEnd := min(len(m.scanApps), showStart+25)

	for i := showStart; i < showEnd; i++ {
		app := m.scanApps[i]
		done := m.scanCur > i
		cur := i == m.scanCur-1

		rowStyle := TableCell
		if cur { rowStyle = TableHighlight }

		if !done && !cur && !app.Skipped {
			continue
		}

		var row string
		if app.Skipped && done {
			row = fmt.Sprintf("  %s  %-28s %s", GlyphSkip, app.Name, BadgeSkipped.Render("no cask"))
		} else if done && !app.Skipped {
			sel := CBUnchecked.Render("○")
			if app.Selected { sel = CBChecked.Render("●") }
			row = fmt.Sprintf("  %s  %-28s %-26s %s  %s",
				GlyphOK, app.Name, app.BestCask, qualityEmoji(app.Score), sel)
		} else if cur {
			row = fmt.Sprintf("  %s  %-28s %s",
				GlyphWait, app.Name, SubtitleStyle.Render("matching…"))
		} else {
			continue
		}
		b.WriteString(rowStyle.Render(row) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpFooter.Render("  matching in progress  •  esc to cancel  •  q quit"))

	return DocStyle.Render(b.String())
}

// ═════════════════════════════════════════════════════════════════
// SELECTION  — uses Huh MultiSelect for polished UX
// ═════════════════════════════════════════════════════════════════

func (m *model) runSelectionForm() tea.Cmd {
	return func() tea.Msg {
		// Build options: checked if score > 90, disabled if skipped
		type appOption struct {
			app      *AppInfo
			label    string
			selected bool
		}
		var opts []huh.Option[*AppInfo]
		for _, app := range m.selApps {
			if app.Skipped {
				// Can't select skipped apps — use Note-like description
				continue
			}
			label := fmt.Sprintf("%s  %s  %s [%d]",
				qualityEmoji(app.Score), app.Name, app.BestCask, app.Score)
			opt := huh.NewOption[*AppInfo](label, app).Selected(app.Selected)
			opts = append(opts, opt)
		}

		var chosen []*AppInfo
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[*AppInfo]().
					Title("📦  Select apps to install").
					Description("space to toggle  •  enter to confirm  •  / to filter").
					Filterable(true).
					Options(opts...).
					Value(&chosen),
			),
		).WithWidth(m.width - 10)

		if err := form.Run(); err != nil {
			return selectionDoneMsg{}
		}
		return selectionDoneMsg{selected: chosen}
	}
}

func (m *model) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Selection is handled by Huh — we only handle navigation while
	// the form result is being processed.
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		m.state = screenMenu
	}
	return m, nil
}

func (m *model) handleSelectionDone(msg selectionDoneMsg) (tea.Model, tea.Cmd) {
	m.selectedApps = msg.selected
	if len(m.selectedApps) == 0 {
		m.state = screenMenu
		return m, nil
	}
	// Confirm with Huh
	var confirmed bool
	err := huh.NewConfirm().
		Title("🚀  Ready to install?").
		Description(fmt.Sprintf("%d app(s) will be installed via Homebrew", len(m.selectedApps))).
		Affirmative("Yes, install!").
		Negative("Review again").
		Value(&confirmed).Run()
	if err != nil || !confirmed {
		m.state = screenMenu
		return m, nil
	}
	return m, m.startInstall()
}

func (m *model) renderSelection() string {
	// During Huh form, we hand over control — show a brief placeholder
	// while the form initializes.
	return DocStyle.Render(fmt.Sprintf("\n  %s  Opening selection…", m.spinner.View()))
}

// ═════════════════════════════════════════════════════════════════
// INSTALLING
// ═════════════════════════════════════════════════════════════════

func (m *model) startInstall() tea.Cmd {
	m.state = screenInstalling
	m.installApps = m.selectedApps
	m.installCur = 0
	m.installTotal = len(m.selectedApps)
	m.installResults = nil
	m.installDone = false
	m.installProg = progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(m.width-20),
		progress.WithoutPercentage(),
	)
	return m.installNext()
}

func (m *model) installNext() tea.Cmd {
	if m.installCur >= m.installTotal {
		return func() tea.Msg { return installDoneMsg{} }
	}
	app := m.installApps[m.installCur]
	return func() tea.Msg {
		ok, out := installCask(app.BestCask)
		msg := ""
		if !ok { msg = out }
		if ok {
			updateHistory(app.Name, "migrated", app.BestCask)
		} else {
			updateHistory(app.Name, "failed", msg)
		}
		return installProgressMsg{idx: m.installCur, name: app.Name, success: ok, err: msg}
	}
}

func (m *model) handleInstallProgress(msg installProgressMsg) (tea.Model, tea.Cmd) {
	m.installResults = append(m.installResults, installResult{name: msg.name, ok: msg.success, err: msg.err})
	m.installCur++
	pct := float64(m.installCur) / float64(m.installTotal)
	return m, tea.Batch(m.installProg.SetPercent(pct), m.installNext())
}

func (m *model) handleInstallDone() (tea.Model, tea.Cmd) {
	m.installDone = true
	m.state = screenSummary
	m.sumOK = 0; m.sumFail = 0; m.sumRows = nil
	for _, r := range m.installResults {
		if r.ok {
			m.sumOK++
			m.sumRows = append(m.sumRows, lipgloss.NewStyle().Foreground(clrSuccess).Render("✓  "+r.name))
		} else {
			m.sumFail++
			m.sumRows = append(m.sumRows, lipgloss.NewStyle().Foreground(clrError).Render("✗  "+r.name+"  ("+r.err+")"))
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

	hdr := TitleStyle.Render(fmt.Sprintf("  🚀  Installing  %s", m.spinner.View()))
	b.WriteString("\n")
	b.WriteString(hdr)
	b.WriteString("\n\n")

	pct := float64(0)
	if m.installTotal > 0 { pct = float64(m.installCur) / float64(m.installTotal) }

	// Progress bar
	bar := m.installProg.ViewAs(pct)
	pctStr := ProgressPct.Render(fmt.Sprintf("%d%%", int(pct*100)))
	b.WriteString(fmt.Sprintf("  %s\n", bar))
	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		pctStr,
		SubtitleStyle.Render(fmt.Sprintf("(%d / %d)", m.installCur, m.installTotal))))

	// Current
	if m.installCur < m.installTotal {
		app := m.installApps[m.installCur]
		b.WriteString(fmt.Sprintf("  %s  %s  →  %s\n\n",
			m.spinner.View(),
			lipgloss.NewStyle().Bold(true).Render(app.Name),
			SubtitleStyle.Render(app.BestCask)))
	}

	// Recent results
	start := max(0, len(m.installResults)-8)
	for _, r := range m.installResults[start:] {
		if r.ok {
			b.WriteString(fmt.Sprintf("  %s  %s\n", GlyphOK, r.name))
		} else {
			b.WriteString(fmt.Sprintf("  %s  %s  (%s)\n", GlyphFail, r.name, r.err))
		}
	}

	b.WriteString("\n")
	b.WriteString(HelpFooter.Render("  installing  •  q to interrupt"))

	return DocStyle.Render(b.String())
}

// ═════════════════════════════════════════════════════════════════
// SUMMARY
// ═════════════════════════════════════════════════════════════════

func (m *model) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Back):
		m.state = screenMenu
	}
	return m, nil
}

func (m *model) renderSummary() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render("  📊  Summary"))
	b.WriteString("\n\n")

	// Stats box
	stats := fmt.Sprintf("\n  %s  %d succeeded\n",
		lipgloss.NewStyle().Foreground(clrSuccess).Bold(true).Render("✓"),
		m.sumOK)
	if m.sumFail > 0 {
		stats += fmt.Sprintf("  %s  %d failed\n",
			lipgloss.NewStyle().Foreground(clrError).Bold(true).Render("✗"),
			m.sumFail)
	}
	stats += "\n"

	for _, row := range m.sumRows {
		stats += fmt.Sprintf("    %s\n", row)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrSurface).
		Padding(1, 2).
		Width(m.width - 8).
		Render(stats)

	b.WriteString(box)
	b.WriteString("\n")
	b.WriteString(HelpFooter.Render("  enter  •  esc  •  q quit"))

	return DocStyle.Render(b.String())
}

// ═════════════════════════════════════════════════════════════════
// HISTORY
// ═════════════════════════════════════════════════════════════════

func (m *model) loadHistory() tea.Cmd {
	m.state = screenHistory
	return func() tea.Msg { return historyLoadedMsg{entries: loadHistory()} }
}

func (m *model) handleHistoryLoaded(msg historyLoadedMsg) (tea.Model, tea.Cmd) {
	m.histEntries = msg.entries
	m.buildHistoryTable()
	return m, nil
}

func (m *model) buildHistoryTable() {
	cols := []table.Column{
		{Title: "App", Width: 28},
		{Title: "Result", Width: 10},
		{Title: "Detail", Width: 40},
		{Title: "When", Width: 25},
	}
	rows := make([]table.Row, 0, len(m.histEntries))
	for _, e := range m.histEntries {
		rows = append(rows, table.Row{e.App, e.Status, e.Detail, e.Timestamp})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][3] > rows[j][3] })

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(min(len(rows)+1, 20)),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).Bold(true)
	s.Cell = s.Cell.Foreground(clrText)
	s.Selected = s.Selected.Background(clrBgLight)
	t.SetStyles(s)
	m.histTable = t
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
	hdr := TitleStyle.Render(fmt.Sprintf("  👀  Migration History  (%d entries)", len(m.histEntries)))
	b.WriteString(hdr)
	b.WriteString("\n\n")

	if len(m.histEntries) > 0 {
		b.WriteString(m.histTable.View())
	} else {
		b.WriteString(SubtitleStyle.Render("  No history yet. Run a scan and install some apps to see results here.\n"))
	}

	b.WriteString("\n")
	b.WriteString(HelpFooter.Render("  esc/enter back  •  q quit"))

	return DocStyle.Render(b.String())
}
