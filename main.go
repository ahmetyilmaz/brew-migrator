package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if !checkBrewInstalled() {
		fmt.Fprintln(os.Stderr, "❌ Homebrew is not installed or not on PATH.")
		fmt.Fprintln(os.Stderr, "   Install from https://brew.sh and try again.")
		os.Exit(1)
	}

	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
