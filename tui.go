package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type screen int

const (
	screenMenu screen = iota
	screenScanning
	screenSelection
	screenReview
	screenInstalling
	screenSummary
	screenHistory
)

type scanStartedMsg struct {
	generation uint64
	apps       []string
	err        error
}
type scanProgressMsg struct {
	generation uint64
	idx        int
	info       AppInfo
}
type scanDoneMsg struct{ generation uint64 }
type installProgressMsg struct {
	name    string
	success bool
	detail  string
}
type installDoneMsg struct{}

type model struct {
	state                         screen
	width, height                 int
	ready, quitting               bool
	menuCur                       int
	scanGeneration                uint64
	scanApps                      []AppInfo
	scanCur, scanTotal            int
	scanDone                      bool
	cursor, reviewCur, historyCur int
	selectedApps                  []*AppInfo
	installApps                   []*AppInfo
	installCur, installTotal      int
	installResults                []installResult
	stopInstall                   bool
	histEntries                   map[string]HistoryEntry
	spinner                       spinner.Model
	err                           error
}
type installResult struct {
	name   string
	ok     bool
	detail string
}

func NewModel() *model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(clrBrand)
	return &model{spinner: s, histEntries: loadHistory()}
}
func (m *model) Init() tea.Cmd { return m.spinner.Tick }
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(0, msg.Width)
		m.height = max(0, msg.Height)
		m.ready = true
		if m.state == screenReview {
			m.reviewCur = min(m.reviewCur, max(0, len(m.reviewLines())-1))
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case scanStartedMsg:
		if m.state != screenScanning || msg.generation != m.scanGeneration {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.state = screenMenu
			return m, nil
		}
		m.scanTotal = len(msg.apps)
		m.scanApps = make([]AppInfo, len(msg.apps))
		for i, name := range msg.apps {
			m.scanApps[i] = AppInfo{Name: name, Score: -1}
		}
		return m, m.scanNext()
	case scanProgressMsg:
		return m.handleScanProgress(msg)
	case scanDoneMsg:
		if m.state != screenScanning || msg.generation != m.scanGeneration {
			return m, nil
		}
		return m.handleScanDone()
	case installProgressMsg:
		m.installResults = append(m.installResults, installResult{msg.name, msg.success, msg.detail})
		m.installCur++
		if m.stopInstall {
			m.state = screenSummary
			return m, nil
		}
		return m, m.installNext()
	case installDoneMsg:
		m.state = screenSummary
		m.histEntries = loadHistory()
	case tea.KeyMsg:
		k := msg.String()
		if m.state == screenInstalling {
			if k == "q" || k == "ctrl+c" {
				m.stopInstall = true
			}
			return m, nil
		}
		if k == "q" || k == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.ready && (m.width < 72 || m.height < 24) {
			return m, nil
		}
		switch m.state {
		case screenMenu:
			switch k {
			case "up", "k":
				m.menuCur = (m.menuCur + 3) % 4
			case "down", "j":
				m.menuCur = (m.menuCur + 1) % 4
			case "enter":
				m.err = nil
				switch m.menuCur {
				case 0:
					return m, m.startScan()
				case 1:
					m.prepareReview()
				case 2:
					m.state = screenHistory
					m.historyCur = 0
					m.histEntries = loadHistory()
				case 3:
					m.quitting = true
					return m, tea.Quit
				}
			}
		case screenScanning:
			if k == "esc" {
				m.scanGeneration++
				m.state = screenMenu
			}
		case screenSelection:
			return m.updateSelection(msg)
		case screenReview:
			switch k {
			case "esc":
				m.state = screenSelection
			case "up", "k":
				m.reviewCur = max(0, m.reviewCur-1)
			case "down", "j":
				m.reviewCur = min(max(0, len(m.reviewLines())-1), m.reviewCur+1)
			case "pgup":
				m.reviewCur = max(0, m.reviewCur-m.listHeight())
			case "pgdown":
				m.reviewCur = min(max(0, len(m.reviewLines())-1), m.reviewCur+m.listHeight())
			case "home":
				m.reviewCur = 0
			case "end":
				m.reviewCur = max(0, len(m.reviewLines())-1)
			case "i":
				if len(m.selectedApps) > 0 {
					return m, m.startInstall()
				}
			}
		case screenSummary:
			switch k {
			case "up", "k":
				m.reviewCur = max(0, m.reviewCur-1)
			case "down", "j":
				m.reviewCur = min(max(0, len(m.installResults)-1), m.reviewCur+1)
			case "enter", "esc":
				m.state = screenMenu
			}
		case screenHistory:
			switch k {
			case "up", "k":
				m.historyCur = max(0, m.historyCur-1)
			case "down", "j":
				m.historyCur = min(max(0, len(m.histEntries)-1), m.historyCur+1)
			case "pgup":
				m.historyCur = max(0, m.historyCur-m.listHeight())
			case "pgdown":
				m.historyCur = min(max(0, len(m.histEntries)-1), m.historyCur+m.listHeight())
			case "esc", "enter":
				m.state = screenMenu
			}
		}
	}
	return m, nil
}

func (m *model) startScan() tea.Cmd {
	m.scanGeneration++
	generation := m.scanGeneration
	m.state = screenScanning
	m.scanCur = 0
	m.scanTotal = 0
	m.scanApps = nil
	m.selectedApps = nil
	m.cursor = 0
	m.scanDone = false
	m.err = nil
	return func() tea.Msg {
		loadInstalledCasks()
		installedCasksMu.Lock()
		err := installedCasksErr
		installedCasksMu.Unlock()
		if err != nil {
			return scanStartedMsg{generation: generation, err: err}
		}
		return scanStartedMsg{generation: generation, apps: scanApplications("/Applications")}
	}
}
func (m *model) scanNext() tea.Cmd {
	generation := m.scanGeneration
	if m.scanCur >= m.scanTotal {
		return func() tea.Msg { return scanDoneMsg{generation} }
	}
	idx := m.scanCur
	name := m.scanApps[idx].Name
	return func() tea.Msg {
		cands := getCaskCandidates(name)
		info := AppInfo{Name: name, Candidates: cands, Score: -1, Skipped: len(cands) == 0}
		if len(cands) > 0 {
			info.BestCask = cands[0].Name
			info.Score = cands[0].Score
			info.Selected = info.Score > 90 && !cands[0].Installed
		}
		return scanProgressMsg{generation: generation, idx: idx, info: info}
	}
}
func (m *model) handleScanProgress(msg scanProgressMsg) (tea.Model, tea.Cmd) {
	if m.state != screenScanning || msg.generation != m.scanGeneration || msg.idx != m.scanCur || msg.idx < 0 || msg.idx >= len(m.scanApps) {
		return m, nil
	}
	m.scanApps[msg.idx] = msg.info
	m.scanCur++
	return m, m.scanNext()
}
func (m *model) handleScanDone() (tea.Model, tea.Cmd) {
	m.scanDone = true
	m.state = screenSelection
	m.cursor = 0
	return m, nil
}
func managed(app AppInfo) bool {
	// A known managed bundle cannot be reinstalled through a different candidate.
	for _, c := range app.Candidates {
		if c.Installed && c.Score > 90 {
			return true
		}
	}
	return false
}
func chosenInstalled(app AppInfo) bool {
	for _, c := range app.Candidates {
		if c.Name == app.BestCask && c.Installed {
			return true
		}
	}
	return false
}
func (m *model) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = screenMenu
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(max(0, len(m.scanApps)-1), m.cursor+1)
	case "pgup":
		m.cursor = max(0, m.cursor-m.selectionHeight())
	case "pgdown":
		m.cursor = min(max(0, len(m.scanApps)-1), m.cursor+m.selectionHeight())
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = max(0, len(m.scanApps)-1)
	case "enter":
		m.prepareReview()
	case " ":
		if m.cursor < len(m.scanApps) {
			a := &m.scanApps[m.cursor]
			if !a.Skipped && !managed(*a) && !chosenInstalled(*a) {
				a.Selected = !a.Selected
			}
		}
	case "left", "right":
		if m.cursor >= len(m.scanApps) {
			break
		}
		a := &m.scanApps[m.cursor]
		if len(a.Candidates) < 2 || managed(*a) {
			break
		}
		idx := 0
		for i, c := range a.Candidates {
			if c.Name == a.BestCask {
				idx = i
				break
			}
		}
		if msg.String() == "right" {
			idx = (idx + 1) % len(a.Candidates)
		} else {
			idx = (idx + len(a.Candidates) - 1) % len(a.Candidates)
		}
		a.BestCask = a.Candidates[idx].Name
		a.Score = a.Candidates[idx].Score
	}
	return m, nil
}
func (m *model) prepareReview() {
	m.selectedApps = nil
	seen := map[string]bool{}
	for i := range m.scanApps {
		a := &m.scanApps[i]
		if a.Selected && !a.Skipped && !managed(*a) && !chosenInstalled(*a) && a.BestCask != "" && !seen[a.BestCask] {
			m.selectedApps = append(m.selectedApps, a)
			seen[a.BestCask] = true
		}
	}
	m.reviewCur = 0
	m.state = screenReview
}
func (m *model) startInstall() tea.Cmd {
	m.state = screenInstalling
	m.installApps = append([]*AppInfo(nil), m.selectedApps...)
	m.installTotal = len(m.installApps)
	m.installCur = 0
	m.installResults = nil
	m.stopInstall = false
	m.reviewCur = 0
	return m.installNext()
}
func (m *model) installNext() tea.Cmd {
	if m.installCur >= m.installTotal {
		return func() tea.Msg { return installDoneMsg{} }
	}
	app := *m.installApps[m.installCur]
	return func() tea.Msg {
		ok, detail := installCask(app.BestCask)
		status := "migrated"
		record := app.BestCask
		if !ok {
			status = "failed"
			record = detail
		} else if detail != "" {
			status = "skipped"
			record = detail
		}
		updateHistory(app.Name, status, record)
		return installProgressMsg{app.Name, ok, detail}
	}
}

const legend = "Legend: * strong match | ~ review match | ? uncertain | - no match"
const selectionLegend = "[x] selected | [ ] available | [B] managed by Brew (locked)"

func matchLabel(score int) string {
	switch {
	case score > 90:
		return "* strong"
	case score >= 50:
		return "~ review"
	case score < 0:
		return "- no match"
	default:
		return "? uncertain"
	}
}
func (m *model) listHeight() int      { return max(1, m.height-9) }
func (m *model) selectionHeight() int { return max(1, m.height-16) }
func visibleRange(cursor, total, rows int) (int, int) {
	rows = max(1, rows)
	cursor = max(0, min(cursor, total-1))
	start := max(0, cursor-rows+1)
	return start, min(total, start+rows)
}
func (m *model) frame(title, subtitle string, body []string, footer string) string {
	width := max(1, m.width)
	height := max(1, m.height)
	trim := func(s string) string { return ansi.Truncate(strings.ReplaceAll(s, "\n", " "), width, "") }
	lines := []string{TitleStyle.Render(trim("BREW MIGRATOR / " + title)), trim(subtitle), strings.Repeat("─", width)}
	available := max(0, height-5)
	for i := 0; i < available; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		lines = append(lines, trim(line))
	}
	lines = append(lines, strings.Repeat("─", width), trim(footer))
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
func (m *model) View() string {
	if !m.ready {
		return "Loading Brew Migrator…"
	}
	if m.width < 72 || m.height < 24 {
		text := fmt.Sprintf("Resize terminal: need 72x24 (now %dx%d). q quits.", m.width, m.height)
		return ansi.Truncate(text, max(1, m.width), "")
	}
	switch m.state {
	case screenScanning:
		return m.renderScanning()
	case screenSelection:
		return m.renderSelection()
	case screenReview:
		return m.renderReview()
	case screenInstalling:
		return m.renderInstalling()
	case screenSummary:
		return m.renderSummary()
	case screenHistory:
		return m.renderHistory()
	default:
		return m.renderMenu()
	}
}
func (m *model) renderMenu() string {
	body := []string{"Scan applications, choose casks, review the plan, then install.", ""}
	labels := []string{"Scan & plan", "Review selected plan", "Migration history", "Quit"}
	for i, label := range labels {
		prefix := "  "
		if i == m.menuCur {
			prefix = "> "
		}
		body = append(body, prefix+label)
	}
	if m.err != nil {
		body = append(body, "", "Error: "+m.err.Error(), "No installations started. Fix Homebrew and scan again.")
	}
	return m.frame("Home", "Local apps → Homebrew casks", body, "↑/↓ move | enter open | q quit")
}
func appRow(a AppInfo, active bool, width int) string {
	mark := "[ ]"
	if a.Selected {
		mark = "[x]"
	}
	if a.Skipped {
		mark = "[-]"
	}
	if managed(a) || chosenInstalled(a) {
		mark = "[B]"
	}
	pointer := " "
	if active {
		pointer = ">"
	}
	nameWidth := max(10, (width-24)/2)
	caskWidth := max(8, width-nameWidth-22)
	name := ansi.Truncate(a.Name, nameWidth, "…")
	cask := a.BestCask
	if cask == "" {
		cask = "—"
	}
	cask = ansi.Truncate(cask, caskWidth, "…")
	name += strings.Repeat(" ", max(0, nameWidth-lipgloss.Width(name)))
	cask += strings.Repeat(" ", max(0, caskWidth-lipgloss.Width(cask)))
	return fmt.Sprintf("%s %s %s  %s %s", pointer, mark, name, cask, matchLabel(a.Score))
}
func (m *model) renderScanning() string {
	body := []string{"Reading installed casks…"}
	if m.scanTotal > 0 {
		body = []string{"    Application                 Chosen cask / match"}
		start, end := visibleRange(max(0, m.scanCur-1), m.scanCur, m.listHeight()-1)
		for i := start; i < end; i++ {
			body = append(body, appRow(m.scanApps[i], i == m.scanCur-1, m.width))
		}
	}
	return m.frame("Scan", fmt.Sprintf("%s Matched %d / %d applications", m.spinner.View(), m.scanCur, m.scanTotal), body, "esc cancel | q quit")
}
func (m *model) renderSelection() string {
	body := []string{"    Application / selection        Chosen cask / confidence"}
	start, end := visibleRange(m.cursor, len(m.scanApps), m.selectionHeight())
	for i := start; i < end; i++ {
		body = append(body, appRow(m.scanApps[i], i == m.cursor, m.width))
	}
	if len(m.scanApps) == 0 {
		body = append(body, "No applications found in /Applications.")
	}
	body = append(body, fmt.Sprintf("Rows %d–%d of %d", min(start+1, len(m.scanApps)), end, len(m.scanApps)), "", legend, selectionLegend)
	if m.cursor < len(m.scanApps) {
		a := m.scanApps[m.cursor]
		body = append(body, "", "Focused: "+a.Name)
		if managed(a) {
			body = append(body, "Already managed by Homebrew. No installation needed.")
		} else if len(a.Candidates) == 0 {
			body = append(body, "No cask found (or search failed). Leave unselected.")
		} else {
			idx := 0
			for i, c := range a.Candidates {
				if c.Name == a.BestCask {
					idx = i
				}
			}
			c := a.Candidates[idx]
			body = append(body, fmt.Sprintf("Cask %d/%d: %s | %s | %s", idx+1, len(a.Candidates), c.Name, c.Tap, c.Version), c.Description)
			// Show alternatives in a viewport centered on the current choice.
			choices := []string{}
			lo, hi := visibleRange(idx, len(a.Candidates), 3)
			for i := lo; i < hi; i++ {
				label := a.Candidates[i].Name
				if i == idx {
					label = "[" + label + "]"
				}
				choices = append(choices, label)
			}
			body = append(body, "Choices: "+strings.Join(choices, " | "))
		}
	}
	return m.frame("Select", fmt.Sprintf("%d applications | space toggles selection; ←/→ changes focused cask", len(m.scanApps)), body, "↑↓ move | ←→ cask | space select | enter review | esc back | q quit")
}
func (m *model) reviewLines() []string {
	lines := []string{}
	for i, app := range m.selectedApps {
		text := fmt.Sprintf("%d. %s → %s", i+1, app.Name, app.BestCask)
		lines = append(lines, strings.Split(ansi.Wrap(text, max(1, m.width), ""), "\n")...)
	}
	return lines
}
func (m *model) renderReview() string {
	body := []string{"Review every package below. Press i to install this exact list.", ""}
	lines := m.reviewLines()
	start, end := visibleRange(m.reviewCur, len(lines), m.listHeight()-2)
	body = append(body, lines[start:end]...)
	if len(m.selectedApps) == 0 {
		body = append(body, "No packages selected. Esc returns to selection.")
	}
	return m.frame("Review installation", fmt.Sprintf("%d packages | lines %d–%d of %d | managed apps excluded", len(m.selectedApps), min(start+1, len(lines)), end, len(lines)), body, "↑↓ / PgUp/PgDn scroll | i install listed packages | esc edit | q quit")
}
func (m *model) renderInstalling() string {
	body := []string{fmt.Sprintf("Completed %d / %d packages", m.installCur, m.installTotal), ""}
	if m.installCur < len(m.installApps) {
		body = append(body, "Current: "+m.installApps[m.installCur].Name+" → "+m.installApps[m.installCur].BestCask)
	}
	start := max(0, len(m.installResults)-max(1, m.listHeight()-4))
	for _, r := range m.installResults[start:] {
		status := "OK"
		if !r.ok {
			status = "FAILED"
		}
		body = append(body, status+"  "+r.name+"  "+r.detail)
	}
	if m.stopInstall {
		body = append(body, "Stopping after the current Homebrew command finishes.")
	}
	return m.frame("Install", m.spinner.View()+" Homebrew is working", body, "q / ctrl+c stop after current package")
}
func (m *model) renderSummary() string {
	body := []string{}
	start, end := visibleRange(m.reviewCur, len(m.installResults), m.listHeight())
	for _, r := range m.installResults[start:end] {
		status := "OK"
		if !r.ok {
			status = "FAILED"
		}
		body = append(body, status+"  "+r.name+"  "+r.detail)
	}
	return m.frame("Results", fmt.Sprintf("Completed %d of %d planned packages", m.installCur, m.installTotal), body, "↑/↓ scroll | enter / esc home | q quit")
}
func (m *model) renderHistory() string {
	entries := make([]HistoryEntry, 0, len(m.histEntries))
	for _, e := range m.histEntries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp > entries[j].Timestamp })
	body := []string{}
	start, end := visibleRange(m.historyCur, len(entries), m.listHeight())
	for _, e := range entries[start:end] {
		body = append(body, fmt.Sprintf("%s | %s | %s | %s", e.App, e.Status, e.Detail, e.Timestamp))
	}
	if len(entries) == 0 {
		body = append(body, "No migration history yet.")
	}
	return m.frame("History", fmt.Sprintf("%d entries, newest first", len(entries)), body, "↑/↓ or PgUp/PgDn scroll | enter / esc home | q quit")
}
