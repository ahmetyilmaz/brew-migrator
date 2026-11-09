package main

import (
	"fmt"
	"io"
	"strings"
)

// Bubble Tea releases the terminal before Run, so Homebrew and sudo have
// exclusive keyboard access. Only process output is captured, never input.
type interactiveInstall struct {
	app    AppInfo
	stdin  io.Reader
	stdout io.Writer
	output strings.Builder
	ok     bool
	detail string
}

func (c *interactiveInstall) SetStdin(r io.Reader)  { c.stdin = r }
func (c *interactiveInstall) SetStdout(w io.Writer) { c.stdout = w }
func (c *interactiveInstall) SetStderr(w io.Writer) {
	if c.stdout == nil {
		c.stdout = w
	}
}
func (c *interactiveInstall) Run() error {
	if c.stdin == nil || c.stdout == nil {
		return fmt.Errorf("interactive installer requires a terminal")
	}
	fmt.Fprintf(c.stdout, "\n📦 Installing %s (%s)\nHomebrew owns the terminal until this package finishes.\nIf sudo asks, use Touch ID if offered, or your Mac password.\nmacOS handles authentication; your input is not logged.\n\n", c.app.Name, c.app.BestCask)
	c.ok, c.detail = installCaskWithInput(c.app.BestCask, c.stdin, func(chunk string) {
		c.output.WriteString(chunk)
		_, _ = io.WriteString(c.stdout, chunk)
	})
	status := "migrated"
	record := c.app.BestCask
	if !c.ok {
		status = "failed"
		record = c.detail
	} else if c.detail != "" {
		status = "skipped"
		record = c.detail
	}
	updateHistory(c.app.Name, status, record)
	return nil
}
