package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// ansiEscape matches CSI sequences (ESC [ ... letter) and simple two-char ESC sequences.
var ansiEscape = regexp.MustCompile(`\x1b(?:\[[0-9;]*[A-Za-z]|[^[])`)

// sanitizeLine decodes a CP437 (Windows OEM) encoded line to UTF-8,
// strips ANSI escape codes, and removes non-printable control characters.
func sanitizeLine(oemBytes []byte) string {
	// Decode CP437 → UTF-8
	decoded, _, err := transform.Bytes(charmap.CodePage437.NewDecoder(), oemBytes)
	if err != nil {
		// Fall back to raw bytes if decode fails
		decoded = oemBytes
	}
	s := string(decoded)

	// Strip ANSI escape sequences
	s = ansiEscape.ReplaceAllString(s, "")

	// Strip non-printable control characters (keep tab)
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (pm *ProcessManager) handleGetProcesses(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	statuses := make([]ProcessStatus, 0, len(pm.order))
	for _, id := range pm.order {
		mp := pm.processes[id]
		mp.mu.Lock()
		statuses = append(statuses, pm.getStatus(mp))
		mp.mu.Unlock()
	}

	writeJSON(w, http.StatusOK, statuses)
}

func (pm *ProcessManager) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	if err := pm.startProcess(mp, true); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (pm *ProcessManager) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	if err := pm.stopProcess(mp); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Disable auto-restart on manual stop and persist to config
	mp.mu.Lock()
	mp.Config.AutoRestart = false
	mp.mu.Unlock()

	pm.mu.Lock()
	for i, pc := range pm.cfg.Processes {
		if pc.ID == id {
			pm.cfg.Processes[i].AutoRestart = false
			break
		}
	}
	pm.cfg.saveConfig(pm.configPath)
	pm.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (pm *ProcessManager) handleToggleAutoRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	var body struct {
		AutoRestart bool `json:"auto_restart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mp.mu.Lock()
	mp.Config.AutoRestart = body.AutoRestart
	mp.mu.Unlock()

	// Persist to config.json
	pm.mu.Lock()
	for i, pc := range pm.cfg.Processes {
		if pc.ID == id {
			pm.cfg.Processes[i].AutoRestart = body.AutoRestart
			break
		}
	}
	pm.cfg.saveConfig(pm.configPath)
	pm.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]bool{"auto_restart": body.AutoRestart})
}

func (pm *ProcessManager) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	// Windows Services don't have a managed log file
	if mp.Config.IsService {
		writeJSON(w, http.StatusOK, map[string][]string{"lines": {}})
		return
	}

	tailN := 30
	if s := r.URL.Query().Get("tail"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 500 {
			tailN = n
		}
	}

	logPath := fmt.Sprintf("./%s.log", id)
	lines, err := tailFile(logPath, tailN)
	if err != nil || lines == nil {
		writeJSON(w, http.StatusOK, map[string][]string{"lines": {}})
		return
	}

	writeJSON(w, http.StatusOK, map[string][]string{"lines": lines})
}

// tailFile reads the last n lines of a file efficiently by seeking from the end.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size == 0 {
		return []string{}, nil
	}

	const maxRead = 131072 // 128 KB — enough for 30 log lines
	offset := size - maxRead
	if offset < 0 {
		offset = 0
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	text := string(buf)
	if offset > 0 {
		// We may have started mid-line; skip to the first newline
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}

	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return []string{}, nil
	}

	// Split into lines, decode each from CP437, strip ANSI and control chars
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		l = strings.TrimRight(l, "\r")
		lines = append(lines, sanitizeLine([]byte(l)))
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines, nil
}

// containsDangerousChars checks if a string contains characters that could be used for command injection
func containsDangerousChars(s string) bool {
	dangerous := []string{"..", "&", "|", ";", ">", "<", "`", "$(", "%", "\n", "\r"}
	for _, char := range dangerous {
		if strings.Contains(s, char) {
			return true
		}
	}
	return false
}

// validateConfig checks that all paths and args in the config are safe
func validateConfig(cfg *Config) error {
	for _, pc := range cfg.Processes {
		if containsDangerousChars(pc.Executable) {
			return fmt.Errorf("invalid executable path: %s", pc.Executable)
		}
		if containsDangerousChars(pc.WorkingDir) {
			return fmt.Errorf("invalid working directory: %s", pc.WorkingDir)
		}
		if pc.IsService && containsDangerousChars(pc.ServiceName) {
			return fmt.Errorf("invalid service name: %s", pc.ServiceName)
		}
		for _, arg := range pc.Args {
			if containsDangerousChars(arg) {
				return fmt.Errorf("invalid argument: %s", arg)
			}
		}
		// Validate log rotation settings
		if pc.LogMaxSizeMB < 0 {
			return fmt.Errorf("log_max_size_mb must be >= 0")
		}
		if pc.LogMaxBackups < 0 {
			return fmt.Errorf("log_max_backups must be >= 0")
		}
		if pc.LogMaxAgeDays < 0 {
			return fmt.Errorf("log_max_age_days must be >= 0")
		}
	}
	return nil
}

func (pm *ProcessManager) handleStartAll(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	errors := make(map[string]string)
	for _, id := range pm.order {
		mp := pm.processes[id]
		if err := pm.startProcess(mp, true); err != nil {
			errors[id] = err.Error()
		}
	}

	if len(errors) > 0 {
		writeJSON(w, http.StatusMultiStatus, map[string]any{
			"status": "partial",
			"errors": errors,
		})
	} else {
		writeJSON(w, http.StatusOK, map[string]string{"status": "all started"})
	}
}

func (pm *ProcessManager) handleStopAll(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	order := make([]string, len(pm.order))
	copy(order, pm.order)
	pm.mu.RUnlock()

	errors := make(map[string]string)

	// Stop in reverse order
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		pm.mu.RLock()
		mp, ok := pm.processes[id]
		pm.mu.RUnlock()

		if !ok {
			continue
		}

		if err := pm.stopProcess(mp); err != nil {
			errors[id] = err.Error()
		}

		// Disable auto-restart on all processes and persist
		mp.mu.Lock()
		mp.Config.AutoRestart = false
		mp.mu.Unlock()

		pm.mu.Lock()
		for j, pc := range pm.cfg.Processes {
			if pc.ID == id {
				pm.cfg.Processes[j].AutoRestart = false
				break
			}
		}
		pm.mu.Unlock()
	}

	// Persist to config once
	pm.mu.Lock()
	pm.cfg.saveConfig(pm.configPath)
	pm.mu.Unlock()

	if len(errors) > 0 {
		writeJSON(w, http.StatusMultiStatus, map[string]any{
			"status": "partial",
			"errors": errors,
		})
	} else {
		writeJSON(w, http.StatusOK, map[string]string{"status": "all stopped"})
	}
}

func (pm *ProcessManager) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	minutes := 5
	if s := r.URL.Query().Get("minutes"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 60 {
			minutes = n
		}
	}

	mp.mu.Lock()
	points := mp.metrics.Last(minutes * 60)
	mp.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

func (pm *ProcessManager) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	data, err := os.ReadFile(pm.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read config")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (pm *ProcessManager) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var cfg Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate config
	if err := validateConfig(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Write to disk
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	if err := os.WriteFile(pm.configPath, data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write config")
		return
	}

	// Update in-memory config
	pm.mu.Lock()
	pm.cfg = &cfg
	pm.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "config updated"})
}

func (pm *ProcessManager) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	events := pm.events.All()
	writeJSON(w, http.StatusOK, events)
}
