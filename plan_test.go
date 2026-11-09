package main

import (
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanSerializationKeepsChoices(t *testing.T) {
	original := AppInfo{Name: "Firefox", BestCask: "firefox@esr", Selected: true, Skipped: true, Candidates: []Candidate{{Name: "firefox"}, {Name: "firefox@esr"}}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored AppInfo
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Selected || !restored.Skipped || restored.BestCask != "firefox@esr" {
		t.Fatal("saved plan loses checkbox/skip state")
	}
}

func TestLastPlanSurvivesRestartAndEdits(t *testing.T) {
	t.Setenv("BREW_MIGRATOR_STATE_DIR", t.TempDir())
	m := NewModel()
	m.state = screenScanning
	m.scanApps = []AppInfo{{Name: "Firefox", BestCask: "firefox", Score: 98, Selected: true, Candidates: []Candidate{{Name: "firefox", Score: 98}, {Name: "firefox@esr", Score: 80}}}, {Name: "Skip", Skipped: true}}
	m.handleScanDone()
	restored := NewModel()
	if restored.state != screenMenu || len(restored.scanApps) != 2 {
		t.Fatal("last scan not restored safely at home")
	}
	restored.menuCur = 1
	_, cmd := restored.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || restored.state != screenReview || len(restored.selectedApps) != 1 {
		t.Fatal("last plan review lost selection or auto-started install")
	}
	restored.Update(tea.KeyMsg{Type: tea.KeyEsc})
	restored.Update(tea.KeyMsg{Type: tea.KeyRight})
	restored.Update(tea.KeyMsg{Type: tea.KeySpace})
	again := NewModel()
	if again.scanApps[0].BestCask != "firefox@esr" || again.scanApps[0].Selected || !again.scanApps[1].Skipped {
		t.Fatal("edited choices not restored")
	}
	info, err := os.Stat(again.planFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("saved plan should be private")
	}
}

func TestCorruptLastPlanIsReported(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BREW_MIGRATOR_STATE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, ".brew_migrator_plan.json"), []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	if m.err == nil || len(m.scanApps) != 0 {
		t.Fatal("corrupt plan silently accepted")
	}
}
