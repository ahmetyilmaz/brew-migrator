package main

import (
	"github.com/charmbracelet/lipgloss"
)

// ── Color palette ──────────────────────────────────────────────────────────
const (
	colorBrand     = "#00BFFF" // deep-sky-blue
	colorGood      = "#00FF7F" // spring-green
	colorWarn      = "#FFD700" // gold
	colorBad       = "#FF4444" // red
	colorMuted     = "#888888"
	colorBg        = "#1A1A2E"
	colorBgLight   = "#16213E"
	colorSurface   = "#0F3460"
	colorText      = "#E0E0E0"
	colorTextDim   = "#666666"
	colorAccent    = "#E94560"
	colorCyan      = "#00CED1"
	colorMagenta   = "#FF69B4"
	colorGreen     = "#32CD32"
	colorYellow    = "#FFD700"
	colorOrange    = "#FF8C00"
)

// ── Reusable styles ────────────────────────────────────────────────────────
var (
	// App / branding
	AppTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorBrand)).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim)).
			MarginBottom(1)

	// Layout
	AppStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Background(lipgloss.Color(colorBg))

	HeaderStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Background(lipgloss.Color(colorSurface))

	// Buttons / Menu
	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2).
			MarginRight(2)

	MenuCursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBad)).
			MarginRight(1)

	MenuSelectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorBrand)).
				Bold(true)

	MenuDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextDim)).
				Italic(true)

	// Quality badges
	BadgePerfectStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorGreen)).
				Bold(true)

	BadgeMaybeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorYellow))

	BadgeQuestionableStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorOrange))

	BadgeSkippedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextDim))

	// Checkboxes
	CheckboxCheckedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorGreen)).
				Bold(true)

	CheckboxUncheckedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextDim))

	CheckboxDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted))

	CursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBrand)).
			Bold(true)

	// Table / list
	ColumnStyle = lipgloss.NewStyle().
			Padding(0, 1)

	RowHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(colorBgLight))

	RowAltStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colorBg))

	// Help
	HelpStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color(colorTextDim)).
			MarginTop(1)

	// Progress
	ProgressBarStyle = lipgloss.NewStyle().
				Padding(0, 2).
				MarginTop(1)

	ProgressPercentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorGood)).
				Bold(true)

	// Status labels
	StatusSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGreen)).
			Bold(true).
			Render("✓")

	StatusFail = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBad)).
			Bold(true).
			Render("✗")

	StatusPending = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim)).
			Render("…")

	StatusSkipped = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Render("—")

	// Borders
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorSurface)).
			Padding(0, 1)

	// Summary / notification
	SummaryGoodStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorGreen)).
				Bold(true)

	SummaryBadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBad)).
			Bold(true)
)

// ── Helpers ────────────────────────────────────────────────────────────────

// QualityBadge returns a lipgloss styled string for a quality level.
func QualityBadge(score int) string {
	switch {
	case score > 90:
		return BadgePerfectStyle.Render("⭐ near perfect")
	case score >= 50:
		return BadgeMaybeStyle.Render("🤔 maybe")
	case score < 0:
		return BadgeSkippedStyle.Render("⏭ skipped")
	default:
		return BadgeQuestionableStyle.Render("❓ questionable")
	}
}

// QualityEmoji returns just the emoji for compact display.
func QualityEmoji(score int) string {
	switch {
	case score > 90:
		return "⭐"
	case score >= 50:
		return "🤔"
	case score < 0:
		return "⏭"
	default:
		return "❓"
	}
}

// QualityPlain returns a plain text quality label (no lipgloss).
func QualityPlain(score int) string {
	switch {
	case score > 90:
		return "⭐ near perfect"
	case score >= 50:
		return "🤔 maybe"
	default:
		return "❓ questionable"
	}
}
