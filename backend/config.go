package main

import (
	"encoding/json"
	"os"
	"time"
)

type ProcessConfig struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Executable      string   `json:"executable"`
	Args            []string `json:"args"`
	WorkingDir      string   `json:"working_dir"`
	AutoRestart     bool     `json:"auto_restart"`
	IsService       bool     `json:"is_service"`
	ServiceName     string   `json:"service_name"`
	Category        string   `json:"category"`
	ShutdownDelay   int      `json:"shutdown_delay"`
	LogMaxSizeMB    int      `json:"log_max_size_mb"`
	LogMaxBackups   int      `json:"log_max_backups"`
	LogMaxAgeDays   int      `json:"log_max_age_days"`
}

type Config struct {
	Processes []ProcessConfig `json:"processes"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *Config) saveConfig(path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// rotateLog handles log file rotation based on size and age.
// Returns the log file path to use.
func rotateLog(logPath string, maxSizeMB, maxBackups, maxAgeDays int) (string, error) {
	if maxSizeMB <= 0 {
		return logPath, nil // No rotation configured
	}

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return logPath, nil // File doesn't exist yet
		}
		return logPath, err
	}

	// Check if file exceeds max size
	maxBytes := int64(maxSizeMB) * 1024 * 1024
	if info.Size() < maxBytes {
		return logPath, nil // Within size limit
	}

	// File too large; rotate it
	// Find the next backup number
	nextNum := 1
	for i := 1; i <= maxBackups; i++ {
		backupPath := logPath + "." + string(rune('0' + i))
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			nextNum = i
			break
		}
		if i == maxBackups {
			nextNum = maxBackups
		}
	}

	// Shift existing backups: .5 -> .6 (remove .6), .4 -> .5, etc.
	for i := nextNum; i > 1; i-- {
		oldPath := logPath + "." + string(rune('0' + i - 1))
		newPath := logPath + "." + string(rune('0' + i))
		os.Rename(oldPath, newPath) // Ignore error if old doesn't exist
	}

	// Move current log to .1
	backupPath := logPath + ".1"
	if err := os.Rename(logPath, backupPath); err != nil {
		return logPath, err
	}

	// Clean up old backups by age if maxAgeDays is set
	if maxAgeDays > 0 {
		now := time.Now()
		for i := 1; i <= maxBackups; i++ {
			checkPath := logPath + "." + string(rune('0' + i))
			if info, err := os.Stat(checkPath); err == nil {
				age := now.Sub(info.ModTime()).Hours() / 24
				if age > float64(maxAgeDays) {
					os.Remove(checkPath)
				}
			}
		}
	}

	return logPath, nil
}
