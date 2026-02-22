import { useState, useEffect } from 'react'
import MetricsChart from './MetricsChart'

function formatUptime(ms) {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${sec}s`
  return `${sec}s`
}

function CpuSparkline({ history }) {
  if (history.length < 2) return null
  const svgH = 28
  const last = history[history.length - 1]
  const strokeColor = last > 80 ? 'var(--red)' : last > 50 ? 'var(--yellow)' : 'var(--green)'
  const points = history
    .map((val, i) => {
      const x = (i / (history.length - 1)) * 100
      const y = svgH - (val / 100) * svgH
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
  return (
    <svg className="sparkline" viewBox={`0 0 100 ${svgH}`} preserveAspectRatio="none" width="100%" height={svgH}>
      <polyline points={points} fill="none" stroke={strokeColor} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

function MemSparkline({ history }) {
  if (history.length < 2) return null
  const svgH = 28
  const peak = Math.max(...history, 1)
  const points = history
    .map((val, i) => {
      const x = (i / (history.length - 1)) * 100
      const y = svgH - (val / peak) * svgH
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
  return (
    <svg className="sparkline" viewBox={`0 0 100 ${svgH}`} preserveAspectRatio="none" width="100%" height={svgH}>
      <polyline points={points} fill="none" stroke="var(--blue)" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  )
}

export default function ProcessCard({ process, onStart, onStop, onToggleAutoRestart, cpuHistory, memHistory }) {
  const {
    id, name, state, pid, cpu, memory_mb, threads,
    started_at, restart_count,
    auto_restart, executable, working_dir, is_service,
  } = process

  const isRunning = state === 'running'
  const isCrashed = state === 'crashed'
  const isStopped = state === 'stopped'

  const cpuClamped = Math.min(cpu, 100)
  const memDisplay = memory_mb >= 1024
    ? `${(memory_mb / 1024).toFixed(1)} GB`
    : `${memory_mb.toFixed(0)} MB`

  const uptime = isRunning && started_at ? formatUptime(Date.now() - started_at) : null

  const [logOpen, setLogOpen] = useState(false)
  const [logLines, setLogLines] = useState([])
  const [logFetched, setLogFetched] = useState(false)
  const [filterText, setFilterText] = useState('')
  const [metricsOpen, setMetricsOpen] = useState(false)

  useEffect(() => {
    if (!logOpen) return
    let cancelled = false

    const fetchLogs = async () => {
      try {
        const res = await fetch(`/api/processes/${id}/logs?tail=100`)
        const data = await res.json()
        if (!cancelled) {
          setLogLines(data.lines ?? [])
          setLogFetched(true)
        }
      } catch { /* ignore */ }
    }

    fetchLogs()
    const interval = setInterval(fetchLogs, 2000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [logOpen, id])

  // Reset log state when panel closes so it reloads fresh next open
  const handleLogToggle = () => {
    if (logOpen) {
      setLogLines([])
      setLogFetched(false)
      setFilterText('')
    }
    setLogOpen(o => !o)
  }

  // Filter log lines
  const filteredLogLines = filterText
    ? logLines.filter(line => line.toLowerCase().includes(filterText.toLowerCase()))
    : logLines

  return (
    <div className={`process-card ${state}`}>
      <div className="card-header">
        <div className="card-title-row">
          <span className={`status-dot dot-${state}`} />
          <h2 className="card-name">{name}</h2>
          <span className={`state-badge badge-${state}`}>
            {isCrashed ? 'CRASHED' : state.toUpperCase()}
          </span>
          {restart_count > 0 && (
            <span
              className="restart-badge"
              title={`Auto-restarted ${restart_count} time${restart_count !== 1 ? 's' : ''}`}
            >
              ↺{restart_count}
            </span>
          )}
        </div>
        <div className="card-meta">
          <span className="meta-label">PID</span>
          <span className="meta-value">{isRunning ? pid : '—'}</span>
          {isRunning && threads > 0 && (
            <>
              <span className="meta-sep">·</span>
              <span className="meta-label">THR</span>
              <span className="meta-value">{threads}</span>
            </>
          )}
          {uptime && (
            <>
              <span className="meta-sep">·</span>
              <span className="meta-label">UP</span>
              <span className="uptime-value">{uptime}</span>
            </>
          )}
        </div>
      </div>

      {isRunning && (
        <div className="stats">
          <div className="stat-row">
            <span className="stat-label">CPU</span>
            <div className="bar-track">
              <div
                className={`bar-fill ${cpuClamped > 80 ? 'bar-hot' : cpuClamped > 50 ? 'bar-warm' : 'bar-cool'}`}
                style={{ width: `${cpuClamped}%` }}
              />
            </div>
            <span className="stat-value">{cpu.toFixed(1)}%</span>
          </div>
          <div className="stat-row">
            <span className="stat-label">MEM</span>
            <div className="bar-track">
              <div
                className="bar-fill bar-mem"
                style={{ width: `${Math.min((memory_mb / 2048) * 100, 100)}%` }}
              />
            </div>
            <span className="stat-value">{memDisplay}</span>
          </div>
        </div>
      )}

      {isRunning && (
        <div className="sparklines">
          <div className="sparkline-row">
            <span className="sparkline-label">CPU</span>
            <CpuSparkline history={cpuHistory} />
          </div>
          <div className="sparkline-row">
            <span className="sparkline-label">MEM</span>
            <MemSparkline history={memHistory} />
          </div>
        </div>
      )}

      <div className="card-path">
        <span className="path-label">EXE</span>
        <span className="path-value" title={executable}>{executable}</span>
      </div>
      {working_dir && (
        <div className="card-path">
          <span className="path-label">DIR</span>
          <span className="path-value" title={working_dir}>{working_dir}</span>
        </div>
      )}

      {!is_service && (
        <div className="log-panel">
          <button className="log-toggle" onClick={handleLogToggle}>
            <span className="log-caret">{logOpen ? '▾' : '▸'}</span>
            Logs
          </button>
          {logOpen && (
            <div className="log-content-wrapper">
              <input
                type="text"
                className="log-filter"
                placeholder="Filter logs..."
                value={filterText}
                onChange={(e) => setFilterText(e.target.value)}
              />
              <div className="log-content">
                {!logFetched ? (
                  <span className="log-empty">Loading…</span>
                ) : filteredLogLines.length === 0 ? (
                  <span className="log-empty">{filterText ? 'No matching log lines.' : 'No log output yet.'}</span>
                ) : (
                  filteredLogLines.map((line, i) => (
                    <div key={i} className="log-line">{line || '\u00a0'}</div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {!is_service && isRunning && (
        <div className="log-panel">
          <button className="log-toggle" onClick={() => setMetricsOpen(!metricsOpen)}>
            <span className="log-caret">{metricsOpen ? '▾' : '▸'}</span>
            Metrics History
          </button>
          {metricsOpen && <MetricsChart processId={id} />}
        </div>
      )}

      <div className="card-footer">
        {is_service ? (
          <span className="service-badge">Windows Service</span>
        ) : (
          <label className="autorestart-toggle" title="Auto-restart on crash">
            <input
              type="checkbox"
              checked={auto_restart}
              onChange={e => onToggleAutoRestart(e.target.checked)}
            />
            <span className="toggle-track">
              <span className="toggle-thumb" />
            </span>
            <span className="toggle-label">Auto-restart</span>
          </label>
        )}

        <div className="card-actions">
          {(isStopped || isCrashed) && (
            <button className="btn btn-start" onClick={onStart}>Start</button>
          )}
          {isRunning && (
            <button className="btn btn-stop" onClick={onStop}>Stop</button>
          )}
        </div>
      </div>
    </div>
  )
}
