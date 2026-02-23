import { useState, useEffect, useRef } from 'react'

function formatLogSize(bytes) {
  if (!bytes) return null
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / 1024).toFixed(1)} KB`
}

export default function LogViewer({ processes, onClose }) {
  const logProcesses = processes.filter(p => !p.is_service)

  const [selectedId, setSelectedId] = useState(logProcesses[0]?.id ?? null)
  const [logLines, setLogLines] = useState([])
  const [logFetched, setLogFetched] = useState(false)
  const [filterText, setFilterText] = useState('')
  const [showScrollBtn, setShowScrollBtn] = useState(false)

  const logRef = useRef(null)
  const userScrolledRef = useRef(false)
  const filterRef = useRef(null)

  const selectedProcess = processes.find(p => p.id === selectedId)

  // Reset + fetch on selection change
  useEffect(() => {
    if (!selectedId) return
    let cancelled = false
    setLogLines([])
    setLogFetched(false)
    userScrolledRef.current = false
    setShowScrollBtn(false)

    const fetchLogs = async () => {
      try {
        const res = await fetch(`/api/processes/${selectedId}/logs?tail=500`)
        const data = await res.json()
        if (!cancelled) {
          setLogLines(data.lines ?? [])
          setLogFetched(true)
        }
      } catch { /* ignore */ }
    }

    fetchLogs()
    const interval = setInterval(fetchLogs, 2000)
    return () => { cancelled = true; clearInterval(interval) }
  }, [selectedId])

  // Auto-scroll to bottom unless user scrolled up
  useEffect(() => {
    const el = logRef.current
    if (!el || userScrolledRef.current) return
    el.scrollTop = el.scrollHeight
  }, [logLines])

  const handleScroll = () => {
    const el = logRef.current
    if (!el) return
    const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
    userScrolledRef.current = !isAtBottom
    setShowScrollBtn(!isAtBottom)
  }

  const scrollToBottom = () => {
    userScrolledRef.current = false
    setShowScrollBtn(false)
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight, behavior: 'smooth' })
  }

  const selectProcess = (id) => {
    setSelectedId(id)
    setFilterText('')
    filterRef.current?.focus()
  }

  // Escape to close
  useEffect(() => {
    const handler = (e) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const filteredLines = filterText
    ? logLines.filter(l => l.toLowerCase().includes(filterText.toLowerCase()))
    : logLines

  const filename = selectedId ? `${selectedId}.log` : ''
  const logPath = selectedProcess?.log_path ?? ''
  const logSize = formatLogSize(selectedProcess?.log_size_bytes)

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="logviewer-modal" onClick={e => e.stopPropagation()}>

        {/* Header */}
        <div className="logviewer-header">
          <h2 className="logviewer-title">Log Viewer</h2>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>

        {/* Process tabs */}
        <div className="logviewer-tabs">
          {logProcesses.map(p => (
            <button
              key={p.id}
              className={`logviewer-tab ${p.id === selectedId ? 'logviewer-tab-active' : ''}`}
              onClick={() => selectProcess(p.id)}
            >
              <span className={`status-dot dot-${p.state}`} />
              {p.name}
            </button>
          ))}
        </div>

        {/* Metadata bar */}
        {selectedProcess && (
          <div className="logviewer-meta">
            <span className="logviewer-meta-item">
              <span className="logviewer-meta-label">File</span>
              <span className="logviewer-meta-value">{filename}</span>
            </span>
            {logPath && (
              <>
                <span className="logviewer-meta-sep" />
                <span className="logviewer-meta-item">
                  <span className="logviewer-meta-label">Path</span>
                  <span className="logviewer-meta-value logviewer-meta-path" title={logPath}>{logPath}</span>
                </span>
              </>
            )}
            {logSize && (
              <>
                <span className="logviewer-meta-sep" />
                <span className="logviewer-meta-item">
                  <span className="logviewer-meta-label">Size</span>
                  <span className="logviewer-meta-value">{logSize}</span>
                </span>
              </>
            )}
            <span className="logviewer-meta-sep" />
            <span className="logviewer-meta-item">
              <span className="logviewer-meta-label">Lines</span>
              <span className="logviewer-meta-value">
                {filteredLines.length}{filterText ? ` of ${logLines.length}` : ''}
              </span>
            </span>
          </div>
        )}

        {/* Filter bar */}
        <div className="logviewer-filter-row">
          <input
            ref={filterRef}
            type="text"
            className="logviewer-filter"
            placeholder="Filter log lines..."
            value={filterText}
            onChange={e => setFilterText(e.target.value)}
          />
          {filterText && (
            <button className="logviewer-filter-clear" onClick={() => setFilterText('')}>×</button>
          )}
        </div>

        {/* Log content */}
        <div className="logviewer-content" ref={logRef} onScroll={handleScroll}>
          {!logFetched ? (
            <span className="log-empty">Loading…</span>
          ) : filteredLines.length === 0 ? (
            <span className="log-empty">
              {filterText ? 'No lines match the filter.' : 'No log output yet.'}
            </span>
          ) : (
            filteredLines.map((line, i) => (
              <div key={i} className="logviewer-line">{line || '\u00a0'}</div>
            ))
          )}
        </div>

        {/* Scroll-to-bottom */}
        {showScrollBtn && (
          <div className="logviewer-scroll-footer">
            <button className="logviewer-scroll-btn" onClick={scrollToBottom}>
              ↓ Scroll to bottom
            </button>
          </div>
        )}

      </div>
    </div>
  )
}
