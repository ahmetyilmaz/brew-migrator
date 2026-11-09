package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type savedPlan struct {
	Version       int       `json:"version"`
	Apps          []AppInfo `json:"apps"`
	HiddenManaged int       `json:"hidden_managed"`
}

func stateDir() string {
	if dir := os.Getenv("BREW_MIGRATOR_STATE_DIR"); dir != "" {
		return dir
	}
	return homeDir()
}
func writePlan(path string, plan savedPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}
func readPlan(path string) (savedPlan, error) {
	var plan savedPlan
	data, err := os.ReadFile(path)
	if err != nil {
		return plan, err
	}
	if err = json.Unmarshal(data, &plan); err != nil {
		return plan, err
	}
	if plan.Version != 1 || plan.Apps == nil {
		return plan, fmt.Errorf("unrecognized saved plan format")
	}
	return plan, nil
}
func (m *model) persistPlan() {
	if m.planFile == "" {
		return
	}
	apps := m.scanApps
	if apps == nil {
		apps = []AppInfo{}
	}
	err := writePlan(m.planFile, savedPlan{Version: 1, Apps: apps, HiddenManaged: m.hiddenManaged})
	if err != nil {
		m.err = fmt.Errorf("could not save last plan: %w", err)
	} else {
		m.err = nil
	}
}
func (m *model) restorePlan() {
	if m.planFile == "" {
		return
	}
	plan, err := readPlan(m.planFile)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		m.err = fmt.Errorf("could not restore last plan: %w", err)
		return
	}
	m.scanApps = plan.Apps
	m.hiddenManaged = plan.HiddenManaged
	m.scanDone = true
	m.scanTotal = len(m.scanApps)
	m.scanCur = m.scanTotal
	m.cursor = 0
}
