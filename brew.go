package main

import (
	"encoding/json"
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

// loadInstalledCasks fetches all installed casks once and caches them.
// The map key is the normalized name (for lookups), the value is the
// original cask name (for installation).
// Subsequent calls are no-ops. Returns the map for convenience.
func loadInstalledCasks() map[string]string {
	installedCasksMu.Lock()
	defer installedCasksMu.Unlock()
	if installedCasksLoaded {
		return installedCasksMap
	}
	installedCasksMap = make(map[string]string)
	// brew list --cask exits non-zero when no casks installed, so accept errors
	out, err := runBrew("list", "--cask")
	if err != nil {
		installedCasksLoaded = true
		return installedCasksMap
	}
	for _, name := range strings.Fields(out) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		// brew list --cask returns short names like "google-chrome"
		// Store normalized → original so we can look up the real cask name
		installedCasksMap[normalizeCaskName(name)] = name
	}
	installedCasksLoaded = true
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
	out, err := cmd.Output()
	if err != nil {
		return "", err
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
		if seen[line] {
			continue
		}
		seen[line] = true
		results = append(results, line)
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

// ── Brew info ─────────────────────────────────────────────────────────────

type brewCaskInfo struct {
	Name        string
	Description string
	Tap         string
	Version     string
	Installed   bool
}

func parseBrewInfo(infoText string) map[string]brewCaskInfo {
	result := make(map[string]brewCaskInfo)
	var current *brewCaskInfo
	var currentName string

	for _, line := range strings.Split(infoText, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "==> ") {
			if current != nil && currentName != "" {
				result[currentName] = *current
			}
			currentName = strings.TrimSpace(line[4:])
			current = &brewCaskInfo{Name: currentName}
			continue
		}
		if current == nil {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			switch key {
			case "Description":
				current.Description = val
			case "Tap":
				current.Tap = val
			case "Versions":
				current.Version = val
			case "Installed":
				current.Installed = val != ""
			}
		}
	}
	if current != nil && currentName != "" {
		result[currentName] = *current
	}
	return result
}

func getCaskCandidates(appName string) []Candidate {
	// Fast path: check if app is already installed as a cask
	if c := fastMatchInstalled(appName); c != nil {
		return []Candidate{*c}
	}

	names := searchCasks(appName)
	if len(names) == 0 {
		return nil
	}

	// Score sort
	type scored struct {
		name  string
		score int
	}
	scoredList := make([]scored, len(names))
	for i, n := range names {
		scoredList[i] = scored{n, scoreCandidate(appName, n)}
	}

	// Official-first sort
	official := make([]scored, 0)
	unofficial := make([]scored, 0)
	for _, s := range scoredList {
		if isOfficialCask(s.name) {
			official = append(official, s)
		} else {
			unofficial = append(unofficial, s)
		}
	}
	sort.Slice(official, func(i, j int) bool { return official[i].score > official[j].score })
	sort.Slice(unofficial, func(i, j int) bool { return unofficial[i].score > unofficial[j].score })

	merged := make([]scored, 0, len(names))
	merged = append(merged, official...)
	merged = append(merged, unofficial...)

	// Fetch info (batched)
	batchSize := 8
	for i := 0; i < len(merged); i += batchSize {
		end := i + batchSize
		if end > len(merged) {
			end = len(merged)
		}
		// Get info for batch
		batchNames := make([]string, 0, end-i)
		for _, s := range merged[i:end] {
			batchNames = append(batchNames, s.name)
		}
		infoText, err := runBrew(append([]string{"info", "--cask"}, batchNames...)...)
		if err != nil {
			continue
		}
		infoMap := parseBrewInfo(infoText)
		for j, s := range merged[i:end] {
			if info, ok := infoMap[s.name]; ok {
				merged[i+j] = scored{s.name, s.score} // keep score, update could be None
				_ = info
			}
		}
	}

	// Build candidates
	candidates := make([]Candidate, 0, len(merged))
	for _, s := range merged {
		c := Candidate{
			Name:      s.name,
			Score:     s.score,
			Official:  isOfficialCask(s.name),
			Installed: isAlreadyInstalled(s.name),
		}
		candidates = append(candidates, c)
	}

	return candidates
}

// ── Install ───────────────────────────────────────────────────────────────

// isAlreadyInstalled checks brew; falls back to in-memory cache if loaded.
func isAlreadyInstalled(pkg string) bool {
	if isInstalledFast(pkg) {
		return true
	}
	return exec.Command("brew", "list", "--cask", pkg).Run() == nil
}

func installCask(name string) (bool, string) {
	cmd := exec.Command("brew", "install", "--cask", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
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
		info.Selected = info.Score > 90
		results = append(results, info)
	}

	return results
}
