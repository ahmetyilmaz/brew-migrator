package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ── Types ─────────────────────────────────────────────────────────────────

// Candidate is a single Homebrew Cask match candidate.
type Candidate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Tap         string `json:"tap"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Score       int    `json:"score"`
	Official    bool   `json:"official"`
}

// AppInfo holds the scan result for a single app bundle.
type AppInfo struct {
	Name       string      `json:"name"`
	BestCask   string      `json:"cask,omitempty"`
	Score      int         `json:"score"`
	Candidates []Candidate `json:"candidates"`
	Selected   bool        `json:"-"`
	Skipped    bool        `json:"-"`
}

// HistoryEntry is one record in the migration history.
type HistoryEntry struct {
	App       string `json:"app"`
	Status    string `json:"status"` // migrated, failed, skipped, ignored
	Detail    string `json:"detail"` // cask name or error message
	Timestamp string `json:"timestamp"`
}

// ── Paths ─────────────────────────────────────────────────────────────────

func homeDir() string {
	usr, err := user.Current()
	if err != nil {
		return os.Getenv("HOME")
	}
	return usr.HomeDir
}

func historyPath() string {
	return filepath.Join(homeDir(), ".brew_migrator_history")
}

// ── Brew list (installed casks cache) ──────────────────────

var (
	installedCasksMap    map[string]string // normalized → original cask name
	installedCasksMu     sync.Mutex
	installedCasksLoaded bool
)

// loadInstalledCasks refreshes the inventory for each scan. Names and app
// artifact targets cover bundles whose display name differs from the cask token.
var installedCasksErr error

type caskJSON struct {
	Token       string                       `json:"token"`
	FullToken   string                       `json:"full_token"`
	Names       []string                     `json:"name"`
	Description string                       `json:"desc"`
	Tap         string                       `json:"tap"`
	Version     string                       `json:"version"`
	Installed   json.RawMessage              `json:"installed"`
	Artifacts   []map[string]json.RawMessage `json:"artifacts"`
}

type caskResponse struct {
	Casks []caskJSON `json:"casks"`
}

func (c caskJSON) isInstalled() bool {
	value := strings.TrimSpace(string(c.Installed))
	return value != "" && value != "null" && value != `""` && value != "false" && value != "[]"
}

func loadInstalledCasks() map[string]string {
	installedCasksMu.Lock()
	defer installedCasksMu.Unlock()
	installedCasksMap = make(map[string]string)
	installedCasksLoaded = false
	out, err := runBrew("info", "--json=v2", "--cask", "--installed")
	var response caskResponse
	if err == nil {
		err = json.Unmarshal([]byte(out), &response)
		if err == nil && response.Casks == nil {
			err = fmt.Errorf("missing casks in Homebrew JSON")
		}
	}
	if err != nil {
		installedCasksErr = fmt.Errorf("cannot read Homebrew inventory: %w", err)
		return installedCasksMap
	}
	for _, c := range response.Casks {
		if !c.isInstalled() {
			continue
		}
		token := c.FullToken
		if token == "" {
			token = c.Token
		}
		aliases := append([]string{c.Token, token}, c.Names...)
		for _, artifact := range c.Artifacts {
			if raw, ok := artifact["app"]; ok {
				var app []json.RawMessage
				if json.Unmarshal(raw, &app) == nil && len(app) > 0 {
					var name string
					if json.Unmarshal(app[0], &name) == nil {
						aliases = append(aliases, strings.TrimSuffix(filepath.Base(name), ".app"))
					}
					if len(app) > 1 {
						var options struct {
							Target string `json:"target"`
						}
						if json.Unmarshal(app[1], &options) == nil {
							aliases = append(aliases, strings.TrimSuffix(filepath.Base(options.Target), ".app"))
						}
					}
				}
				var target string
				if json.Unmarshal(artifact["target"], &target) == nil && target != "" {
					aliases = append(aliases, strings.TrimSuffix(filepath.Base(target), ".app"))
				}
			}
		}
		for _, alias := range aliases {
			if key := normalizeCaskName(alias); key != "" {
				installedCasksMap[key] = token
			}
		}
	}
	installedCasksLoaded = true
	installedCasksErr = nil
	return installedCasksMap
}

// isInstalledFast checks the in-memory map; no shell call.
func isInstalledFast(caskName string) bool {
	installedCasksMu.Lock()
	defer installedCasksMu.Unlock()
	if installedCasksMap == nil {
		return false
	}
	_, ok := installedCasksMap[normalizeCaskName(caskName)]
	return ok
}

// fastMatchInstalled checks if the app name (normalized) directly matches
// an installed cask. Returns a Candidate with the real cask name if found.
func fastMatchInstalled(appName string) *Candidate {
	normalized := normalizeCaskName(appName)
	if normalized == "" {
		return nil
	}
	installedCasksMu.Lock()
	realName, ok := installedCasksMap[normalized]
	installedCasksMu.Unlock()
	if !ok {
		return nil
	}
	return &Candidate{
		Name:      realName, // original cask name, NOT normalized
		Score:     100,
		Installed: true,
	}
}

func normalizeCaskName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func checkBrewInstalled() bool {
	return exec.Command("brew", "--version").Run() == nil
}

// ── App scanner ───────────────────────────────────────────────────────────

func scanApplications(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var apps []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") {
			apps = append(apps, strings.TrimSuffix(e.Name(), ".app"))
		}
	}
	sort.Strings(apps)
	return apps
}

// ── Brew search ───────────────────────────────────────────────────────────

func runBrew(args ...string) (string, error) {
	cmd := exec.Command("brew", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("brew %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func searchCasks(appName string) []string {
	out, err := runBrew("search", "--cask", appName)
	if err != nil {
		return nil
	}
	var results []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "==>") || strings.HasPrefix(line, "#") {
			continue
		}
		for _, name := range strings.Fields(line) {
			if name == "✔" || name == "✓" || seen[name] {
				continue
			}
			seen[name] = true
			results = append(results, name)
		}
	}
	return results
}

// ── Matching / scoring ────────────────────────────────────────────────────

func isOfficialCask(name string) bool {
	n := strings.ToLower(name)
	if n == "" {
		return false
	}
	if strings.HasPrefix(n, "homebrew/cask/") ||
		strings.HasPrefix(n, "homebrew/") {
		return true
	}
	if strings.HasPrefix(n, "homebrew/cask-versions/") ||
		strings.HasPrefix(n, "homebrew/cask-fonts/") {
		return true
	}
	if !strings.Contains(n, "/") {
		return true
	}
	return false
}

func scoreCandidate(appName, candidate string) int {
	base := strings.ToLower(strings.TrimSpace(appName))
	cand := strings.ToLower(strings.TrimSpace(candidate))
	score := 0

	if cand == base {
		score += 50
	}
	if strings.HasPrefix(cand, base+"/") {
		score += 30
	}
	if strings.HasSuffix(cand, "/"+base) {
		score += 20
	}
	tail := cand
	if idx := strings.LastIndex(cand, "/"); idx >= 0 {
		tail = cand[idx+1:]
	}
	if base == tail {
		score += 40
	} else if strings.Contains(tail, base) {
		score += max(10, 20-abs(len(base)-len(tail)))
	}

	if isOfficialCask(candidate) {
		score += 5
	}

	// Penalty for longer names
	penalty := max(0, len(cand)-len(base)) / 4
	score -= penalty

	return score
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ── Brew candidates ─────────────────────────────────────────────────────
func getCaskCandidates(appName string) []Candidate {
	if c := fastMatchInstalled(appName); c != nil {
		return []Candidate{*c}
	}
	names := searchCasks(appName)
	if len(names) == 0 {
		return nil
	}
	out, err := runBrew(append([]string{"info", "--json=v2", "--cask"}, names...)...)
	if err != nil {
		return nil
	}
	var response caskResponse
	if json.Unmarshal([]byte(out), &response) != nil {
		return nil
	}
	candidates := make([]Candidate, 0, len(response.Casks))
	for _, c := range response.Casks {
		name := c.FullToken
		if name == "" {
			name = c.Token
		}
		score := scoreCandidate(appName, name)
		for _, label := range append(c.Names, c.Token) {
			if normalizeCaskName(label) == normalizeCaskName(appName) {
				score = max(score, 95)
			}
		}
		candidates = append(candidates, Candidate{Name: name, Description: c.Description, Tap: c.Tap, Version: c.Version, Installed: c.isInstalled() || isInstalledFast(name), Score: score, Official: isOfficialCask(name)})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Official && !candidates[j].Official
	})
	return candidates
}

// ── Install ───────────────────────────────────────────────────────────────

func installCask(name string) (bool, string) {
	// Read a fresh inventory immediately before installation; never use a stale
	// scan to decide whether an app needs installing.
	out, err := runBrew("list", "--cask")
	if err != nil {
		return false, "Cannot verify installed casks: " + err.Error()
	}
	for _, token := range strings.Fields(out) {
		if normalizeCaskName(token) == normalizeCaskName(name) || token == filepath.Base(name) {
			return true, "Already managed by Homebrew; skipped"
		}
	}
	cmd := exec.Command("brew", "install", "--cask", "--adopt", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return false, detail
	}
	return true, ""
}

// ── History ───────────────────────────────────────────────────────────────

var historyMu sync.Mutex

func loadHistory() map[string]HistoryEntry {
	historyMu.Lock()
	defer historyMu.Unlock()

	path := historyPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]HistoryEntry)
	}
	var entries map[string]HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return make(map[string]HistoryEntry)
	}
	return entries
}

func saveHistory(entries map[string]HistoryEntry) {
	historyMu.Lock()
	defer historyMu.Unlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	path := historyPath()
	tmp := path + ".tmp"
	_ = os.WriteFile(tmp, data, 0644)
	_ = os.Rename(tmp, path)
}

func updateHistory(app, status, detail string) {
	entries := loadHistory()
	entries[app] = HistoryEntry{
		App:       app,
		Status:    status,
		Detail:    detail,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	saveHistory(entries)
}

// ── Application scanning + matching (full pipeline) ───────────────────────

func matchAllApps(appNames []string) []AppInfo {
	results := make([]AppInfo, 0, len(appNames))

	for _, name := range appNames {
		info := AppInfo{Name: name}
		candidates := getCaskCandidates(name)
		if len(candidates) == 0 {
			info.Score = -1
			info.Skipped = true
			results = append(results, info)
			continue
		}
		info.Candidates = candidates
		info.BestCask = candidates[0].Name
		info.Score = candidates[0].Score
		info.Selected = info.Score > 90 && !candidates[0].Installed
		results = append(results, info)
	}

	return results
}
