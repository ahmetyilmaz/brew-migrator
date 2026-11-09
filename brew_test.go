package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubBrew(t *testing.T, script string) {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "brew"), []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", d)
	installedCasksMu.Lock()
	installedCasksMap = nil
	installedCasksLoaded = false
	installedCasksMu.Unlock()
}

func TestManagedAppsNotSelected(t *testing.T) {
	stubBrew(t, `case "$1" in
 list) echo appcleaner ;;
 info) echo '{"casks":[{"token":"appcleaner","name":["AppCleaner"],"installed":"3.6","artifacts":[{"app":["AppCleaner.app"]}]}]}' ;;
 esac
 `)
	loadInstalledCasks()
	apps := matchAllApps([]string{"AppCleaner"})
	if apps[0].Selected {
		t.Fatal("already managed AppCleaner selected for installation")
	}
}

func TestInstalledArtifactAlias(t *testing.T) {
	stubBrew(t, `case "$1" in
 list) echo vendor-cleaner ;;
 info) echo '{"casks":[{"token":"vendor-cleaner","name":["Vendor Cleaner"],"installed":"3.6","artifacts":[{"app":["dist/Cleaner.app"],"target":"/Applications/AppCleaner.app"}]}]}' ;;
 esac
 `)
	loadInstalledCasks()
	c := fastMatchInstalled("AppCleaner")
	if c == nil || c.Name != "vendor-cleaner" || !c.Installed {
		t.Fatalf("artifact alias not matched: %+v", c)
	}
}

func TestInventoryRefreshAndFailure(t *testing.T) {
	stubBrew(t, `echo '{"casks":[{"token":"appcleaner","installed":"3.6","name":["AppCleaner"]}]}'`)
	loadInstalledCasks()
	if fastMatchInstalled("AppCleaner") == nil {
		t.Fatal("first inventory missing")
	}
	stubBrew(t, `echo '{"casks":[]}'`)
	loadInstalledCasks()
	if fastMatchInstalled("AppCleaner") != nil {
		t.Fatal("removed app remained cached")
	}
	stubBrew(t, `echo 'inventory failed' >&2; exit 1`)
	m := &model{}
	cmd := m.startScan()
	m.Update(cmd())
	if m.state != screenMenu || m.err == nil {
		t.Fatal("inventory failure must return home with visible error")
	}
}

func TestInstallSkipsManagedAndFailsClosed(t *testing.T) {
	stubBrew(t, `case "$1" in
 list) echo appcleaner ;;
 install) echo 'INSTALL MUST NOT RUN'; exit 9 ;;
 esac`)
	ok, detail := installCask("appcleaner")
	if !ok || detail != "Already managed by Homebrew; skipped" {
		t.Fatalf("managed install: %v %s", ok, detail)
	}
	stubBrew(t, `case "$1" in
 list) exit 1 ;;
 install) echo 'INSTALL MUST NOT RUN'; exit 9 ;;
 esac`)
	ok, detail = installCask("appcleaner")
	if ok || !strings.Contains(detail, "Cannot verify installed casks") {
		t.Fatalf("inventory failure: %v %s", ok, detail)
	}
}

func TestSearchParsesMultipleColumnsAndJSONMetadata(t *testing.T) {
	stubBrew(t, `case "$1" in
 search) printf '==> Casks\nalpha  alpha-beta\n' ;;
 info) echo '{"casks":[{"token":"alpha","name":["Alpha"],"desc":"Stable app","version":"1","tap":"homebrew/cask","installed":null},{"token":"alpha-beta","name":["Alpha Beta"],"installed":null}]}' ;;
 esac`)
	c := getCaskCandidates("Alpha")
	if len(c) != 2 || c[0].Name != "alpha" || c[0].Description != "Stable app" || c[0].Version != "1" {
		t.Fatalf("candidates=%+v", c)
	}
}

func TestMalformedInventoryBlocksScan(t *testing.T) {
	stubBrew(t, `echo '{"unexpected":[]}'`)
	m := &model{}
	cmd := m.startScan()
	m.Update(cmd())
	if m.err == nil || m.state != screenMenu {
		t.Fatal("malformed inventory accepted as empty")
	}
}

func TestInstallAdoptsExistingArtifacts(t *testing.T) {
	stubBrew(t, `case "$*" in
 'list --cask') exit 0 ;;
 'install --cask --adopt alpha') exit 0 ;;
 *) echo 'unexpected command'; exit 9 ;;
 esac`)
	ok, detail := installCask("alpha")
	if !ok || detail != "" {
		t.Fatalf("existing artifacts must use Homebrew adoption: %v %s", ok, detail)
	}
}
