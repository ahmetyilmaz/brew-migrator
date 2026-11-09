package main

import "github.com/charmbracelet/lipgloss"

// ── Velvet palette ────────────────────────────────
// These are adaptive — look great on both light and dark terminals.
var (
	clrBg      = lipgloss.AdaptiveColor{Light: "#f4f4f5", Dark: "#1a1b26"}
	clrBgLight = lipgloss.AdaptiveColor{Light: "#e4e4e7", Dark: "#24283b"}
	clrSurface = lipgloss.AdaptiveColor{Light: "#d4d4d8", Dark: "#2f3340"}
	clrBrand   = lipgloss.AdaptiveColor{Light: "#7149b5", Dark: "#c4a3ff"}
	clrSuccess = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#9ece6a"}
	clrWarn    = lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#e0af68"}
	clrError   = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f7768e"}
	clrText    = lipgloss.AdaptiveColor{Light: "#18181b", Dark: "#c0caf5"}
	clrTextDim = lipgloss.AdaptiveColor{Light: "#71717a", Dark: "#565f89"}
	clrMuted   = lipgloss.AdaptiveColor{Light: "#a1a1aa", Dark: "#3b4261"}
	clrAccent  = lipgloss.AdaptiveColor{Light: "#a77bc4", Dark: "#6d548e"}
)

// ── Reusable style blocks ───────────────────────────────────────

var (
	// Outer document wrapper
	DocStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrSurface).
			Padding(1, 2)

	// Title bar
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrBrand)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(clrTextDim)

	// ── Sidebar / Menu ──

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(clrSurface).
			Padding(0, 2).
			MarginBottom(1)

	ActiveCardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(clrBrand).
			BorderLeft(true).
			Padding(0, 2).
			MarginBottom(1).
			Background(clrBgLight)

	MenuItemTitle = lipgloss.NewStyle().
			Foreground(clrText).
			Padding(0, 1)

	MenuItemTitleActive = lipgloss.NewStyle().
				Foreground(clrBrand).
				Bold(true).
				Padding(0, 1)

	MenuItemDesc = lipgloss.NewStyle().
			Foreground(clrTextDim).
			Italic(true).
			Padding(0, 1)

	CursorArrow = lipgloss.NewStyle().
			Foreground(clrBrand).
			Bold(true)

	// ── Buttons ──

	BtnPrimary = lipgloss.NewStyle().
			Background(clrBrand).
			Foreground(clrBg).
			Bold(true).
			Padding(0, 2).
			MarginRight(1)

	BtnSecondary = lipgloss.NewStyle().
			Background(clrSurface).
			Foreground(clrText).
			Padding(0, 2).
			MarginRight(1)

	// ── Tables ──

	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrBrand).
			Padding(0, 1)

	TableCell = lipgloss.NewStyle().
			Padding(0, 1)

	TableHighlight = lipgloss.NewStyle().
			Background(clrBgLight)

	// ── Progress ──

	ProgressBar = lipgloss.NewStyle().
			Padding(0, 1)

	ProgressPct = lipgloss.NewStyle().
			Foreground(clrSuccess).
			Bold(true)

	// ── Help footer ──

	HelpFooter = lipgloss.NewStyle().
			Foreground(clrTextDim).
			Padding(0, 2).
			MarginTop(1)

	// ── Quality badges ──

	BadgePerfect = lipgloss.NewStyle().
			Foreground(clrSuccess).
			Bold(true)

	BadgeMaybe = lipgloss.NewStyle().
			Foreground(clrWarn)

	BadgeQuestionable = lipgloss.NewStyle().
				Foreground(clrError)

	BadgeSkipped = lipgloss.NewStyle().
			Foreground(clrMuted)

	// ── Checkbox / selection ──

	CBChecked = lipgloss.NewStyle().
			Foreground(clrSuccess).
			Bold(true)

	CBUnchecked = lipgloss.NewStyle().
			Foreground(clrTextDim)

	CBDisabled = lipgloss.NewStyle().
			Foreground(clrMuted)

	// ── Status glyphs ──

	GlyphOK   = lipgloss.NewStyle().Foreground(clrSuccess).Bold(true).Render("✓")
	GlyphFail = lipgloss.NewStyle().Foreground(clrError).Bold(true).Render("✗")
	GlyphWait = lipgloss.NewStyle().Foreground(clrTextDim).Render("…")
	GlyphSkip = lipgloss.NewStyle().Foreground(clrMuted).Render("—")

	// ── Separator ──

	Sep = lipgloss.NewStyle().
		Foreground(clrSurface).
		Render("──────────────────────────────────────────")
)

// ── Quality helpers ─────────────────────────────────────────────

func qualityBadge(score int) string {
	switch {
	case score > 90:
		return BadgePerfect.Render("⭐ near perfect")
	case score >= 50:
		return BadgeMaybe.Render("🤔 maybe")
	case score < 0:
		return BadgeSkipped.Render("⏭ skipped")
	default:
		return BadgeQuestionable.Render("❓ questionable")
	}
}

func qualityEmoji(score int) string {
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
