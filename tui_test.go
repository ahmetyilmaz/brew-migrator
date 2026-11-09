package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
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
	if m.state != screenSelection || selection == nil {
		t.Fatal("completed scan did not open selection")
	}
}
