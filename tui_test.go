package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanViewNarrowTerminal(t *testing.T) {
	for _, width := range []int{0, 1, 8, 10, 80} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("width %d: %v", width, r)
				}
			}()
			m := &model{state: screenScanning, ready: true, width: width}
			if m.View() == "" {
				t.Error("empty scan view")
			}
		}()
	}
}

func TestCancelledScanIgnoresPendingMessages(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "brew"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", d)
	for _, restart := range []bool{false, true} {
		m := &model{state: screenScanning, scanTotal: 1, scanApps: []AppInfo{{Name: "Firefox"}}}
		pending := m.scanNext()
		m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if restart {
			m.state = screenScanning
		}
		before := m.state
		_, cmd := m.Update(pending())
		if m.scanCur != 0 || cmd != nil {
			t.Errorf("restart %v: cancelled result advanced scan", restart)
		}
		m.Update(scanDoneMsg{})
		if m.state != before {
			t.Errorf("restart %v: stale completion changed screen", restart)
		}
	}
}

func TestActiveScanAdvancesToSelection(t *testing.T) {
	m := &model{state: screenScanning, scanTotal: 1, scanGeneration: 3, scanApps: []AppInfo{{Name: "Firefox"}}}
	_, cmd := m.Update(scanProgressMsg{generation: 3, idx: 0, info: AppInfo{Name: "Firefox", BestCask: "firefox"}})
	if m.scanCur != 1 || cmd == nil {
		t.Fatal("active scan did not advance")
	}
	_, selection := m.Update(cmd())
	if m.state != screenSelection || selection != nil {
		t.Fatal("completed scan did not open selection")
	}
}

func TestSelectionNavigationAndAlternatives(t *testing.T) {
	m := &model{state: screenSelection, scanApps: []AppInfo{
		{Name: "Alpha", BestCask: "alpha", Candidates: []Candidate{{Name: "alpha", Score: 95}, {Name: "alpha-beta", Score: 60}}},
		{Name: "Beta", BestCask: "beta", Candidates: []Candidate{{Name: "beta", Score: 95}, {Name: "beta-dev", Score: 60}}},
	}}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.scanApps[0].BestCask != "alpha" || m.scanApps[1].BestCask != "beta-dev" {
		t.Fatal("candidate choice did not affect only focused row")
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.scanApps[1].Selected || m.scanApps[0].Selected {
		t.Fatal("space must toggle only focused row")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if !m.scanApps[1].Selected {
		t.Fatal("navigation changed selection")
	}
}

func TestReviewListsEveryPackageBeforeInstall(t *testing.T) {
	m := &model{state: screenSelection, width: 100, height: 24, ready: true}
	for i := 0; i < 35; i++ {
		m.scanApps = append(m.scanApps, AppInfo{Name: fmt.Sprintf("App %02d", i), BestCask: fmt.Sprintf("cask-%02d", i), Selected: true})
	}
	m.scanApps = append(m.scanApps, AppInfo{Name: "Managed", BestCask: "managed", Selected: true, Candidates: []Candidate{{Name: "managed", Installed: true, Score: 100}}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != screenReview || cmd != nil || len(m.selectedApps) != 35 {
		t.Fatal("review must exclude managed apps and not start installation")
	}
	for i := 0; i < 35; i++ {
		m.reviewCur = i
		if !strings.Contains(m.View(), fmt.Sprintf("App %02d → cask-%02d", i, i)) {
			t.Fatalf("package %d not reachable in review", i)
		}
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != screenReview || cmd != nil {
		t.Fatal("enter accidentally installed from review")
	}
}

func TestViewsFitTerminalAndKeepLegend(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {160, 50}, {48, 12}, {40, 12}, {8, 3}} {
		for _, state := range []screen{screenMenu, screenScanning, screenSelection, screenReview, screenInstalling, screenSummary, screenHistory} {
			m := &model{state: state, width: size[0], height: size[1], ready: true}
			for i := 0; i < 100; i++ {
				m.scanApps = append(m.scanApps, AppInfo{Name: strings.Repeat("Long application ", 8), BestCask: "some-cask", Candidates: []Candidate{{Name: "some-cask"}}})
			}
			view := m.View()
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("state %v at %v overflows: %dx%d", state, size, lipgloss.Width(view), lipgloss.Height(view))
			}
			if state == screenSelection && size[0] >= 80 && !strings.Contains(view, "Legend:") {
				t.Fatal("selection legend missing")
			}
		}
	}
}

func TestSelectionManagedAppLocked(t *testing.T) {
	m := &model{state: screenSelection, scanApps: []AppInfo{{Name: "AppCleaner", BestCask: "appcleaner", Candidates: []Candidate{{Name: "appcleaner", Installed: true, Score: 100}}}}}
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.scanApps[0].Selected {
		t.Fatal("managed app selectable")
	}
}

func TestUnrelatedInstalledCandidateDoesNotLockApplication(t *testing.T) {
	m := &model{state: screenSelection, scanApps: []AppInfo{{Name: "Alpha", BestCask: "alpha", Candidates: []Candidate{{Name: "alpha", Score: 95}, {Name: "alpha-tool", Installed: true, Score: 15}}}}}
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.scanApps[0].Selected {
		t.Fatal("unrelated installed search result locked app")
	}
}

func TestSmallTerminalCannotStartHiddenInstall(t *testing.T) {
	m := &model{state: screenReview, ready: true, width: 30, height: 8, selectedApps: []*AppInfo{{Name: "Alpha", BestCask: "alpha"}}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.state != screenReview || cmd != nil {
		t.Fatal("installed behind resize message")
	}
}

func TestReviewKeepsLongCaskVisible(t *testing.T) {
	m := &model{state: screenReview, ready: true, width: 80, height: 24, selectedApps: []*AppInfo{{Name: strings.Repeat("A", 150), BestCask: "vendor/tap/actual-package"}}}
	if !strings.Contains(m.View(), "vendor/tap/actual-package") {
		t.Fatal("long app name hides package identifier")
	}
}

func TestReviewWidenAfterScrolling(t *testing.T) {
	m := &model{state: screenReview, ready: true, width: 72, height: 24, selectedApps: []*AppInfo{{Name: strings.Repeat("Long App ", 800), BestCask: "vendor/tap/package"}}}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	view := m.View()
	if !strings.Contains(view, "vendor/tap/package") {
		t.Fatal("focused review row lost after resize")
	}
}
