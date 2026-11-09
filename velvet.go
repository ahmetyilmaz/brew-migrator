package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func velvetPanel(title string, lines []string, width, height int) string {
	innerWidth := max(1, width-4)
	content := []string{lipgloss.NewStyle().Foreground(clrBrand).Bold(true).Render(ansi.Truncate(title, innerWidth, "…"))}
	content = append(content, lines...)
	innerHeight := max(1, height-2)
	for len(content) < innerHeight {
		content = append(content, "")
	}
	if len(content) > innerHeight {
		content = content[:innerHeight]
	}
	for i := range content {
		content[i] = ansi.Truncate(content[i], innerWidth, "…")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(clrAccent).Padding(0, 1).Width(max(1, width-2)).Render(strings.Join(content, "\n"))
}
func (m *model) renderSelection() string {
	split := m.width >= 104
	leftWidth := m.width
	panelHeight := m.selectionHeight()*2 + 3
	if split {
		leftWidth = (m.width - 3) * 53 / 100
		panelHeight = max(7, m.height-11)
	}
	start, end := visibleRange(m.cursor, len(m.scanApps), m.selectionHeight())
	rows := []string{}
	for i := start; i < end; i++ {
		a := m.scanApps[i]
		marker := "☐"
		if a.Selected {
			marker = "☑"
		}
		if managed(a) || chosenInstalled(a) {
			marker = "🍺"
		}
		if a.Skipped {
			marker = "⏭"
		}
		pointer := "  "
		if i == m.cursor {
			pointer = "› "
		}
		first := pointer + marker + "  " + a.Name
		second := "     " + a.BestCask + "  ·  " + matchLabel(a.Score)
		if a.Skipped {
			second = "     No package match"
		}
		if managed(a) {
			second = "     Already Brew-managed · locked"
		}
		if i == m.cursor {
			highlight := lipgloss.NewStyle().Background(clrBgLight).Foreground(clrText)
			first = highlight.Render(ansi.Truncate(first, max(1, leftWidth-4), "…"))
			second = highlight.Render(ansi.Truncate(second, max(1, leftWidth-4), "…"))
		}
		rows = append(rows, first, second)
	}
	if len(m.scanApps) == 0 {
		message := "No applications found."
		if m.hiddenManaged > 0 {
			message = "✨ All scanned apps are already Brew-managed."
		}
		rows = append(rows, message)
	}
	left := velvetPanel(fmt.Sprintf("📦 Applications · %d–%d of %d", min(start+1, len(m.scanApps)), end, len(m.scanApps)), rows, leftWidth, panelHeight)
	inspector := []string{}
	rightWidth := m.width
	detailHeight := 5
	if split {
		rightWidth = m.width - leftWidth - 3
		detailHeight = panelHeight
	}
	if m.cursor < len(m.scanApps) {
		a := m.scanApps[m.cursor]
		if split {
			inspector = append(inspector, lipgloss.NewStyle().Bold(true).Render(a.Name), matchLabel(a.Score), "")
		}
		if managed(a) {
			inspector = append(inspector, "🍺 Already managed by Homebrew.", "No installation needed.")
		} else if len(a.Candidates) == 0 {
			inspector = append(inspector, "⏭ No cask found (or search failed).")
		} else {
			idx := 0
			for i, c := range a.Candidates {
				if c.Name == a.BestCask {
					idx = i
				}
			}
			c := a.Candidates[idx]
			// Put the full focused identifier before supplementary metadata;
			// alternatives may share a long tap prefix, so ellipses alone are ambiguous.
			inspector = nil
			if split {
				inspector = append(inspector, a.Name, matchLabel(a.Score), "")
			}
			full := strings.Split(ansi.Wrap("◉ "+c.Name, max(1, rightWidth-4), ""), "\n")
			inspector = append(inspector, full...)
			budget := detailHeight - 3
			lo, hi := visibleRange(idx, len(a.Candidates), 3)
			for i := lo; i < hi; i++ {
				if i != idx && len(inspector) < budget {
					inspector = append(inspector, "○ "+a.Candidates[i].Name)
				}
			}
			if split && len(inspector)+2 <= budget {
				inspector = append(inspector, "", c.Description)
			}
			if split && c.Tap != "" && len(inspector)+1 <= budget {
				inspector = append(inspector, "🏷  "+c.Tap+" · "+c.Version)
			}

		}
	}
	inspectorTitle := "🔎 Package inspector"
	if m.cursor < len(m.scanApps) && len(m.scanApps[m.cursor].Candidates) > 0 {
		app := m.scanApps[m.cursor]
		idx := 0
		for i, c := range app.Candidates {
			if c.Name == app.BestCask {
				idx = i
			}
		}
		inspectorTitle += fmt.Sprintf(" · %d/%d", idx+1, len(app.Candidates))
	}
	right := velvetPanel(inspectorTitle, inspector, rightWidth, detailHeight)
	var content string
	if split {
		content = lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right)
	} else {
		content = left + "\n" + right
	}
	body := strings.Split(content, "\n")
	body = append(body, "", legend, selectionLegend)
	selected := 0
	for _, a := range m.scanApps {
		if a.Selected && !managed(a) && !chosenInstalled(a) {
			selected++
		}
	}
	return m.frame("Choose your brew", fmt.Sprintf("✓ Scan  ● Choose  ◇ Review  ↗ Migrate · %d selected · %d hidden", selected, m.hiddenManaged), body, "↑↓ move  ←→ cask  space select  enter review  esc back  q quit")
}

func (m *model) logText() string {
	if m.installLog.Len() > 0 {
		return m.installLog.String()
	}
	var b strings.Builder
	for _, r := range m.installResults {
		status := "✅ "
		if !r.ok {
			status = "❌ "
		}
		b.WriteString(status + r.name + "\n" + r.detail + "\n\n")
	}
	return b.String()
}
func (m *model) logLines() []string {
	text := ansi.Strip(m.logText())
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return -1
		}
		return r
	}, text)
	return strings.Split(ansi.Hardwrap(text, max(1, m.width-4), true), "\n")
}
func (m *model) logHeight() int { return max(1, m.height-12) }
func (m *model) scrollLog(key string) {
	maxOffset := max(0, len(m.logLines())-m.logHeight())
	if m.followLog {
		m.logOffset = maxOffset
	}
	switch key {
	case "up", "k":
		m.logOffset = max(0, m.logOffset-1)
		m.followLog = false
	case "down", "j":
		m.logOffset = min(maxOffset, m.logOffset+1)
		m.followLog = false
	case "pgup":
		m.logOffset = max(0, m.logOffset-m.logHeight())
		m.followLog = false
	case "pgdown":
		m.logOffset = min(maxOffset, m.logOffset+m.logHeight())
		m.followLog = false
	case "home":
		m.logOffset = 0
		m.followLog = false
	case "end", "f":
		m.logOffset = maxOffset
		m.followLog = true
	}
}
func (m *model) renderLog(installing bool) string {
	lines := m.logLines()
	maxOffset := max(0, len(lines)-m.logHeight())
	offset := min(maxOffset, max(0, m.logOffset))
	if m.followLog {
		offset = maxOffset
	}
	end := min(len(lines), offset+m.logHeight())
	follow := "⏸ reading earlier output"
	if m.followLog {
		follow = "↳ following latest output"
	}
	title := "📜 Complete installation log"
	body := []string{fmt.Sprintf("%d / %d packages complete   •   %s", m.installCur, m.installTotal, follow), ""}
	panel := velvetPanel(title, lines[offset:end], m.width, m.logHeight()+4)
	body = append(body, strings.Split(panel, "\n")...)
	body = append(body, fmt.Sprintf("Log lines %d–%d of %d · wraps to your window", offset+1, end, len(lines)))
	footer := "↑↓ / PgUp/PgDn scroll  home start  end follow  enter home  q quit"
	heading := "Results"
	subtitle := "✨ All output stays available here — including full errors."
	if installing {
		heading = "Brewing your apps"
		subtitle = m.spinner.View() + " Homebrew output · live"
		footer = "↑↓ / PgUp/PgDn scroll  home start  end follow  q stop after package"
		if m.installCur < len(m.installApps) {
			subtitle += "  📦 " + m.installApps[m.installCur].Name
		}
		if m.stopInstall {
			subtitle = "⏳ Stopping after the current Homebrew command finishes."
		}
	}
	return m.frame(heading, subtitle, body, footer)
}
func (m *model) renderInstalling() string { return m.renderLog(true) }
func (m *model) renderSummary() string    { return m.renderLog(false) }
