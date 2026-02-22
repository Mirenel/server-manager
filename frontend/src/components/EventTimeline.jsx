import { useEffect, useState } from 'react'

export default function EventTimeline() {
  const [open, setOpen] = useState(false)
  const [events, setEvents] = useState([])

  useEffect(() => {
    if (!open) return

    const fetchEvents = async () => {
      try {
        const res = await fetch('/api/events')
        if (res.ok) {
          const data = await res.json()
          setEvents(data || [])
        }
      } catch {
        // ignore fetch errors
      }
    }

    fetchEvents()
    const interval = setInterval(fetchEvents, 5000)
    return () => clearInterval(interval)
  }, [open])

  // Reverse chronological order (newest first)
  const sortedEvents = [...events].reverse()

  const formatTime = (ms) => {
    const date = new Date(ms)
    return date.toLocaleTimeString('en-US', { hour12: false })
  }

  const getDotClass = (type) => {
    return `timeline-dot ${type}`
  }

  return (
    <section className="timeline-section">
      <button
        className="timeline-toggle"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span className="timeline-caret">▼</span>
        Event Timeline
      </button>

      {open && (
        <div className="timeline-list">
          {sortedEvents.length === 0 ? (
            <div style={{ padding: '10px 18px', color: 'var(--text-muted)', fontSize: '12px' }}>
              No events yet
            </div>
          ) : (
            sortedEvents.map((evt, idx) => (
              <div key={idx} className="timeline-item">
                <span className="timeline-time">
                  {formatTime(evt.timestamp_ms)}
                </span>
                <div className={getDotClass(evt.type)} />
                <span className="timeline-name">{evt.process_name}</span>
                <span className="timeline-type">{evt.type}</span>
              </div>
            ))
          )}
        </div>
      )}
    </section>
  )
}
