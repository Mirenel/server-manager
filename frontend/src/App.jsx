import { useEffect, useState, useRef, useCallback } from 'react'
import ProcessCard from './components/ProcessCard'
import Toast from './components/Toast'
import ConfigEditor from './components/ConfigEditor'
import ComparisonView from './components/ComparisonView'
import EventTimeline from './components/EventTimeline'

const WS_URL = 'ws://localhost:8090/ws'
const RECONNECT_DELAY = 3000
const HISTORY_MAX = 30

export default function App() {
  const [processes, setProcesses] = useState([])
  const [connected, setConnected] = useState(false)
  const [toasts, setToasts] = useState([])
  const [theme, setTheme] = useState(() => localStorage.getItem('theme') ?? 'dark')
  const [configOpen, setConfigOpen] = useState(false)
  const [compareOpen, setCompareOpen] = useState(false)
  const wsRef = useRef(null)
  const reconnectTimer = useRef(null)
  const cpuHistoryRef = useRef({})   // { [id]: number[] } — rolling 30 CPU samples
  const memHistoryRef = useRef({})   // { [id]: number[] } — rolling 30 memory (MB) samples
  const prevStatesRef = useRef({})   // { [id]: string }  — previous state for crash detection

  // Set theme on root element and localStorage
  useEffect(() => {
    document.documentElement.dataset.theme = theme === 'light' ? 'light' : ''
    localStorage.setItem('theme', theme)
  }, [theme])

  const dismissToast = useCallback((id) => {
    setToasts(t => t.filter(x => x.id !== id))
  }, [])

  const connect = useCallback(() => {
    const ws = new WebSocket(WS_URL)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      clearTimeout(reconnectTimer.current)
    }

    ws.onmessage = (e) => {
      try {
        const updated = JSON.parse(e.data)

        updated.forEach(proc => {
          const { id, state, cpu, memory_mb, name } = proc

          // CPU sparkline history
          const cpuHist = cpuHistoryRef.current[id] ?? []
          cpuHistoryRef.current[id] = [...cpuHist, cpu].slice(-HISTORY_MAX)

          // Memory sparkline history
          const memHist = memHistoryRef.current[id] ?? []
          memHistoryRef.current[id] = [...memHist, memory_mb].slice(-HISTORY_MAX)

          // Crash toast
          const prevState = prevStatesRef.current[id]
          if (prevState && prevState !== 'crashed' && state === 'crashed') {
            setToasts(t => [...t, { id: crypto.randomUUID(), name }])
          }
          prevStatesRef.current[id] = state
        })

        setProcesses(updated)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      setConnected(false)
      reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY)
    }

    ws.onerror = () => ws.close()
  }, [])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])

  async function handleStart(id) {
    await fetch(`/api/processes/${id}/start`, { method: 'POST' })
  }

  async function handleStop(id) {
    await fetch(`/api/processes/${id}/stop`, { method: 'POST' })
  }

  async function handleToggleAutoRestart(id, value) {
    await fetch(`/api/processes/${id}/autorestart`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ auto_restart: value }),
    })
  }

  async function handleStartAll() {
    await fetch('/api/processes/start-all', { method: 'POST' })
  }

  async function handleStopAll() {
    await fetch('/api/processes/stop-all', { method: 'POST' })
  }

  const runningCount = processes.filter(p => p.state === 'running').length
  const totalCpu = processes
    .filter(p => p.state === 'running')
    .reduce((s, p) => s + p.cpu, 0)
  const totalMemMb = processes
    .filter(p => p.state === 'running')
    .reduce((s, p) => s + p.memory_mb, 0)
  const totalMemDisplay = totalMemMb >= 1024
    ? `${(totalMemMb / 1024).toFixed(1)} GB`
    : `${totalMemMb.toFixed(0)} MB`

  // Group processes by category
  const groups = {}
  const categoryOrder = []
  processes.forEach(proc => {
    const cat = proc.category || 'uncategorized'
    if (!groups[cat]) {
      groups[cat] = []
      categoryOrder.push(cat)
    }
    groups[cat].push(proc)
  })

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <h1 className="app-title">Server Manager</h1>
          <span className="process-summary">{runningCount} / {processes.length} running</span>
        </div>
        {runningCount > 0 && (
          <div className="header-stats">
            <span className="header-stat">
              <span className="header-stat-label">CPU</span>
              <span className="header-stat-value">{totalCpu.toFixed(1)}%</span>
            </span>
            <span className="header-stat-sep" />
            <span className="header-stat">
              <span className="header-stat-label">RAM</span>
              <span className="header-stat-value">{totalMemDisplay}</span>
            </span>
          </div>
        )}
        <div className="header-right">
          <div className="header-controls">
            <button className="header-btn" onClick={handleStartAll} title="Start all processes">
              Start All
            </button>
            <button className="header-btn" onClick={handleStopAll} title="Stop all processes">
              Stop All
            </button>
            <button className="header-btn" onClick={() => setCompareOpen(true)} title="Compare process metrics">
              Compare
            </button>
            <button className="header-btn" onClick={() => setConfigOpen(true)} title="Edit configuration">
              Config
            </button>
            <button
              className="theme-toggle"
              onClick={() => setTheme(t => t === 'light' ? 'dark' : 'light')}
              title="Toggle theme"
            >
              {theme === 'light' ? '🌙' : '☀️'}
            </button>
          </div>
          <div className={`ws-badge ${connected ? 'ws-connected' : 'ws-disconnected'}`}>
            <span className="ws-dot" />
            {connected ? 'Live' : 'Reconnecting...'}
          </div>
        </div>
      </header>

      <main className="process-main">
        {processes.length === 0 && (
          <div className="empty-state">
            {connected ? 'No processes configured.' : 'Connecting to backend...'}
          </div>
        )}
        {categoryOrder.map(cat => (
          <div key={cat} className="process-group">
            <div className="group-heading">{cat}</div>
            <div className="process-group-items">
              {groups[cat].map(proc => (
                <ProcessCard
                  key={proc.id}
                  process={proc}
                  onStart={() => handleStart(proc.id)}
                  onStop={() => handleStop(proc.id)}
                  onToggleAutoRestart={(val) => handleToggleAutoRestart(proc.id, val)}
                  cpuHistory={cpuHistoryRef.current[proc.id] ?? []}
                  memHistory={memHistoryRef.current[proc.id] ?? []}
                />
              ))}
            </div>
          </div>
        ))}
      </main>

      <EventTimeline />

      {configOpen && <ConfigEditor onClose={() => setConfigOpen(false)} />}
      {compareOpen && (
        <ComparisonView
          processes={processes}
          cpuHistoryRef={cpuHistoryRef}
          memHistoryRef={memHistoryRef}
          onClose={() => setCompareOpen(false)}
        />
      )}

      <Toast toasts={toasts} onDismiss={dismissToast} />
    </div>
  )
}
