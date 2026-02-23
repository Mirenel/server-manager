package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type ProcessState string

const (
	StateRunning  ProcessState = "running"
	StateStopped  ProcessState = "stopped"
	StateCrashed  ProcessState = "crashed"
	StateStopping ProcessState = "stopping"
)

type ManagedProcess struct {
	Config           ProcessConfig
	State            ProcessState
	PID              int32
	CPU              float64
	MemoryRSS        uint64
	Threads          int32
	StartedAt        time.Time
	RestartCount     int
	StoppingDeadline time.Time // when force kill will happen (zero if not stopping)
	cmd              *exec.Cmd
	mu               sync.Mutex
	manualStop       bool
	metrics          *MetricsRingBuffer
}

type ProcessStatus struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	State            ProcessState `json:"state"`
	PID              int32        `json:"pid"`
	CPU              float64      `json:"cpu"`
	MemoryMB         float64      `json:"memory_mb"`
	Threads          int32        `json:"threads"`
	StartedAt        int64        `json:"started_at"`        // unix ms, 0 if not running
	StoppingDeadline int64        `json:"stopping_deadline"` // unix ms, 0 if not stopping
	RestartCount     int          `json:"restart_count"`
	AutoRestart      bool         `json:"auto_restart"`
	Executable       string       `json:"executable"`
	WorkingDir       string       `json:"working_dir"`
	IsService        bool         `json:"is_service"`
	Category         string       `json:"category"`
}

type ProcessManager struct {
	processes  map[string]*ManagedProcess
	order      []string
	mu         sync.RWMutex
	hub        *WSHub
	configPath string
	cfg        *Config
	events     *EventStore
}

func newProcessManager(cfg *Config, configPath string) *ProcessManager {
	pm := &ProcessManager{
		processes:  make(map[string]*ManagedProcess),
		order:      make([]string, 0, len(cfg.Processes)),
		hub:        newWSHub(),
		configPath: configPath,
		cfg:        cfg,
		events:     &EventStore{},
	}
	for _, pc := range cfg.Processes {
		pm.processes[pc.ID] = &ManagedProcess{
			Config:  pc,
			State:   StateStopped,
			metrics: &MetricsRingBuffer{},
		}
		pm.order = append(pm.order, pc.ID)
	}
	return pm
}

func (pm *ProcessManager) run() {
	go pm.hub.run()
	go pm.monitor()
}

// ── Windows Service helpers ──────────────────────────────────────────────────

// queryServiceStatus uses `sc queryex` to get the running state and PID.
func queryServiceStatus(serviceName string) (state ProcessState, pid int32, err error) {
	out, cmdErr := exec.Command("sc", "queryex", serviceName).Output()
	if cmdErr != nil {
		return StateStopped, 0, cmdErr
	}
	output := string(out)

	if strings.Contains(output, "STOP_PENDING") {
		state = StateStopping
	} else if strings.Contains(output, "RUNNING") {
		state = StateRunning
	} else {
		state = StateStopped
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PID") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				pidStr := strings.TrimSpace(parts[1])
				if p, parseErr := strconv.ParseInt(pidStr, 10, 32); parseErr == nil {
					pid = int32(p)
				}
			}
			break
		}
	}
	return
}

func (pm *ProcessManager) startServiceProcess(mp *ManagedProcess) error {
	mp.mu.Lock()
	if mp.State == StateRunning {
		mp.mu.Unlock()
		return nil
	}
	serviceName := mp.Config.ServiceName
	mp.mu.Unlock()

	// Run net start without holding the lock — monitor loop will detect RUNNING state
	out, err := exec.Command("net", "start", serviceName).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "already been started") {
			pm.events.Record(mp.Config.ID, mp.Config.Name, EventStarted)
			return nil
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	pm.events.Record(mp.Config.ID, mp.Config.Name, EventStarted)
	return nil
}

func (pm *ProcessManager) stopServiceProcess(mp *ManagedProcess) error {
	mp.mu.Lock()
	if mp.State != StateRunning {
		mp.mu.Unlock()
		return nil
	}
	mp.State = StateStopping
	serviceName := mp.Config.ServiceName
	mp.mu.Unlock()

	// Run net stop without holding the lock — monitor loop will detect STOPPED state
	out, err := exec.Command("net", "stop", serviceName).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "not started") {
			mp.mu.Lock()
			mp.State = StateStopped
			mp.mu.Unlock()
			return nil
		}
		// net stop can fail/timeout even when the service is still shutting down.
		// Query actual state instead of blindly reverting to StateRunning.
		actualState, _, scErr := queryServiceStatus(serviceName)
		mp.mu.Lock()
		if scErr == nil {
			mp.State = actualState // could be StateStopping, StateStopped, or StateRunning
		} else {
			mp.State = StateRunning // can't determine, revert
		}
		mp.mu.Unlock()
		return fmt.Errorf("%w: %s", err, msg)
	}
	pm.events.Record(mp.Config.ID, mp.Config.Name, EventStopped)
	return nil
}

// ── Regular process helpers ──────────────────────────────────────────────────

// startExecProcess starts a managed executable process.
// manualStart=true resets the restart counter; false increments it (auto-restart path).
func (pm *ProcessManager) startExecProcess(mp *ManagedProcess, manualStart bool) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.State == StateRunning {
		return nil
	}

	if manualStart {
		mp.RestartCount = 0
	} else {
		mp.RestartCount++
	}
	mp.StartedAt = time.Now()

	cmd := exec.Command(mp.Config.Executable, mp.Config.Args...)
	if mp.Config.WorkingDir != "" {
		cmd.Dir = mp.Config.WorkingDir
	}

	// Redirect stdout/stderr to separate log files for each process
	logPath := fmt.Sprintf("./%s.log", mp.Config.ID)

	// Apply log rotation if configured
	if mp.Config.LogMaxSizeMB > 0 {
		_, err := rotateLog(logPath, mp.Config.LogMaxSizeMB, mp.Config.LogMaxBackups, mp.Config.LogMaxAgeDays)
		if err != nil {
			return fmt.Errorf("failed to rotate log: %w", err)
		}
	}

	logFile, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a pipe for stdin so the process can read but gets no input
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		logFile.Close()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	cmd.Stdin = stdinRead
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		stdinRead.Close()
		stdinWrite.Close()
		return err
	}

	// Close the read end in parent; keep write end open so process can read indefinitely
	stdinRead.Close()

	mp.cmd = cmd
	mp.PID = int32(cmd.Process.Pid)
	mp.State = StateRunning
	mp.manualStop = false
	pm.events.Record(mp.Config.ID, mp.Config.Name, EventStarted)

	go func() {
		cmd.Wait()
		logFile.Close()
		stdinWrite.Close()

		mp.mu.Lock()
		wasManual := mp.manualStop
		if wasManual {
			mp.State = StateStopped
		} else {
			mp.State = StateCrashed
		}
		mp.PID = 0
		mp.CPU = 0
		mp.MemoryRSS = 0
		mp.Threads = 0
		mp.StartedAt = time.Time{}
		mp.StoppingDeadline = time.Time{}
		shouldRestart := !wasManual && mp.Config.AutoRestart
		mp.mu.Unlock()

		if wasManual {
			pm.events.Record(mp.Config.ID, mp.Config.Name, EventStopped)
		} else {
			pm.events.Record(mp.Config.ID, mp.Config.Name, EventCrashed)
		}

		if shouldRestart {
			log.Printf("[auto-restart] %s crashed — restarting in 3s", mp.Config.Name)
			time.Sleep(3 * time.Second)
			if err := pm.startExecProcess(mp, false); err != nil {
				log.Printf("[auto-restart] failed to restart %s: %v", mp.Config.Name, err)
			}
		} else if !wasManual {
			log.Printf("[crash] %s exited unexpectedly (auto-restart off)", mp.Config.Name)
		}
	}()

	return nil
}

func (pm *ProcessManager) stopExecProcess(mp *ManagedProcess) error {
	// Capture state while holding lock, then release before polling/sleeping
	mp.mu.Lock()
	if mp.State != StateRunning || mp.cmd == nil || mp.cmd.Process == nil {
		mp.mu.Unlock()
		return nil
	}

	mp.manualStop = true
	pid := mp.PID
	proc := mp.cmd.Process
	delay := mp.Config.ShutdownDelay

	// Set stopping state so frontend shows countdown
	mp.State = StateStopping
	if delay > 0 {
		mp.StoppingDeadline = time.Now().Add(time.Duration(delay) * time.Second)
	}
	mp.mu.Unlock()

	// If no delay, kill immediately
	if delay == 0 {
		return proc.Kill()
	}

	// Graceful shutdown with timeout
	pidStr := strconv.FormatInt(int64(pid), 10)
	softKillCmd := exec.Command("taskkill", "/PID", pidStr)
	_ = softKillCmd.Run() // Soft kill, ignore errors

	// Poll for process exit with 500ms interval
	pollInterval := 500 * time.Millisecond
	maxWait := time.Duration(delay) * time.Second
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		// Check if process is still alive via gopsutil
		if p, err := process.NewProcess(pid); err == nil {
			running, err := p.IsRunning()
			if err == nil && !running {
				// Process exited gracefully
				return nil
			}
		}

		time.Sleep(pollInterval)
		elapsed += pollInterval
	}

	// Still running after timeout; force kill
	log.Printf("[shutdown] %s did not exit gracefully after %ds; forcing kill", mp.Config.Name, delay)
	return proc.Kill()
}

// ── Public start / stop ──────────────────────────────────────────────────────

func (pm *ProcessManager) startProcess(mp *ManagedProcess, manualStart bool) error {
	if mp.Config.IsService {
		return pm.startServiceProcess(mp)
	}
	return pm.startExecProcess(mp, manualStart)
}

func (pm *ProcessManager) stopProcess(mp *ManagedProcess) error {
	if mp.Config.IsService {
		return pm.stopServiceProcess(mp)
	}
	return pm.stopExecProcess(mp)
}

// ── Monitor loop ─────────────────────────────────────────────────────────────

func (pm *ProcessManager) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		pm.mu.RLock()
		statuses := make([]ProcessStatus, 0, len(pm.order))

		for _, id := range pm.order {
			mp := pm.processes[id]
			mp.mu.Lock()

			if mp.Config.IsService {
				// Services: poll sc queryex each tick for live state + PID
				state, pid, err := queryServiceStatus(mp.Config.ServiceName)
				if err == nil {
					if mp.State == StateStopping {
						// Respect stopping state: only transition when fully stopped
						if state == StateStopped {
							mp.State = StateStopped
							mp.PID = 0
							pm.events.Record(mp.Config.ID, mp.Config.Name, EventStopped)
						} else {
							// STOP_PENDING or still RUNNING during shutdown — keep stopping
							mp.PID = pid
						}
					} else {
						mp.State = state
						mp.PID = pid
					}
				}
				if mp.State != StateRunning && mp.State != StateStopping {
					mp.CPU = 0
					mp.MemoryRSS = 0
					mp.Threads = 0
				}
			}

			// CPU / memory / threads via gopsutil (also during stopping — process is still alive)
			if (mp.State == StateRunning || mp.State == StateStopping) && mp.PID > 0 {
				p, err := process.NewProcess(mp.PID)
				if err == nil {
					cpu, _ := p.CPUPercent()
					mem, _ := p.MemoryInfo()
					threads, _ := p.NumThreads()

					mp.CPU = cpu
					if mem != nil {
						mp.MemoryRSS = mem.RSS
					}
					mp.Threads = threads

					// Push metrics to ring buffer
					mp.metrics.Push(cpu, float64(mp.MemoryRSS)/1024/1024)
				}
			}

			statuses = append(statuses, pm.getStatus(mp))
			mp.mu.Unlock()
		}

		pm.mu.RUnlock()
		pm.hub.broadcast(statuses)
	}
}

func (pm *ProcessManager) getStatus(mp *ManagedProcess) ProcessStatus {
	var startedAt int64
	if !mp.StartedAt.IsZero() {
		startedAt = mp.StartedAt.UnixMilli()
	}
	var stoppingDeadline int64
	if !mp.StoppingDeadline.IsZero() {
		stoppingDeadline = mp.StoppingDeadline.UnixMilli()
	}
	return ProcessStatus{
		ID:               mp.Config.ID,
		Name:             mp.Config.Name,
		State:            mp.State,
		PID:              mp.PID,
		CPU:              mp.CPU,
		MemoryMB:         float64(mp.MemoryRSS) / 1024 / 1024,
		Threads:          mp.Threads,
		StartedAt:        startedAt,
		StoppingDeadline: stoppingDeadline,
		RestartCount:     mp.RestartCount,
		AutoRestart:      mp.Config.AutoRestart,
		Executable:       mp.Config.Executable,
		WorkingDir:       mp.Config.WorkingDir,
		IsService:        mp.Config.IsService,
		Category:         mp.Config.Category,
	}
}
