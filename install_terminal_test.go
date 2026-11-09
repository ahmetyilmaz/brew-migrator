package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestInteractiveInstallUsesInputAndDoesNotLogIt(t *testing.T) {
	t.Setenv("BREW_MIGRATOR_STATE_DIR", t.TempDir())
	stubBrew(t, `case "$1" in
 list) exit 0 ;;
 install)
 printf 'Authentication requested\n'
 IFS= read -r answer
 if [ "$answer" = 'FAKE_TEST_INPUT' ]; then printf 'Installation finished\n';exit 0;fi
 printf 'Missing terminal input\n';exit 1 ;;
 esac`)
	var terminal bytes.Buffer
	job := &interactiveInstall{app: AppInfo{Name: "Test", BestCask: "test"}}
	job.SetStdin(strings.NewReader("FAKE_TEST_INPUT\n"))
	job.SetStdout(&terminal)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	if !job.ok || !strings.Contains(terminal.String(), "Installation finished") {
		t.Fatalf("interactive input failed: %s", terminal.String())
	}
	if strings.Contains(job.output.String(), "FAKE_TEST_INPUT") {
		t.Fatal("input appeared in captured output")
	}
	if !strings.Contains(job.output.String(), "Authentication requested") {
		t.Fatal("output not retained for Velvet")
	}
}

func TestTerminalHandoffErrorStaysInLog(t *testing.T) {
	m := &model{state: screenInstalling, stopInstall: true}
	m.Update(installProgressMsg{name: "App", detail: "could not release terminal"})
	if !strings.Contains(m.installLog.String(), "could not release terminal") {
		t.Fatal("terminal handoff error omitted from log")
	}
}
