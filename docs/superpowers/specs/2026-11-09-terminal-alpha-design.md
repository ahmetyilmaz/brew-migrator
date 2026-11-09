# Terminal alpha redesign

Mr. Worldwide chose to rebuild the existing terminal interface for Ghostty.

Use one Bubble Tea program for every screen. Nested Huh programs currently
compete for input and redraws. Replace them with a bounded list and a separate
candidate detail panel. Rows show application, chosen cask, confidence, and
selection or managed status. Up/down moves focus; left/right chooses a cask;
space selects; enter opens a full, scrollable package review. Installation
requires an explicit `i` from that review. Show the legend and available keys.
All screens must fit the reported terminal width and height, including resize.

Refresh installed casks before each scan using Homebrew JSON names, tokens,
and app artifact targets. Managed apps remain visible and cannot be selected.
Recheck the installed state immediately before each installation. Failures to
read inventory must be visible and block installation rather than look empty.
Keep all metadata local. Do not install or uninstall real applications during QA.

Alternatives: a separate macOS GUI was offered; the user chose the terminal.
Embedding Huh as a child model is possible, but direct list controls make package
alternatives and the required plan review easier to keep consistent.

Release: test resize, navigation, plan review, metadata aliases, failures, and
managed-app exclusion. Build both macOS architectures, commit and push, publish
the next available alpha, and observe CI and attached release artifacts.
