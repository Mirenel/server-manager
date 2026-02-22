import { useEffect, useState } from 'react'

export default function ConfigEditor({ onClose }) {
  const [config, setConfig] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)

  // Auto-dismiss success message after 3 seconds
  useEffect(() => {
    if (!success) return
    const timer = setTimeout(() => setSuccess(false), 3000)
    return () => clearTimeout(timer)
  }, [success])

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const res = await fetch('/api/config')
        if (res.ok) {
          const text = await res.text()
          // Pretty-print JSON
          const parsed = JSON.parse(text)
          setConfig(JSON.stringify(parsed, null, 2))
        } else {
          setError('Failed to load config')
        }
      } catch (err) {
        setError('Failed to parse config')
      } finally {
        setLoading(false)
      }
    }

    fetchConfig()
  }, [])

  const handleSave = async () => {
    setError('')
    setSuccess(false)

    // Client-side validation
    try {
      const parsed = JSON.parse(config)
      if (!parsed.processes) {
        setError('Config must have a "processes" array')
        return
      }
    } catch (err) {
      setError(`Invalid JSON: ${err.message}`)
      return
    }

    // Send to backend
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: config,
      })

      if (res.ok) {
        setSuccess(true)
      } else {
        const errData = await res.json()
        setError(errData.error || 'Failed to save config')
      }
    } catch (err) {
      setError('Network error saving config')
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">Edit Configuration</h2>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>

        <div className="modal-body">
          {success && (
            <div className="config-restart-banner">
              ✓ Configuration saved successfully. Backend will use the new config immediately.
            </div>
          )}
          {error && (
            <div className="config-error">
              ✕ {error}
            </div>
          )}

          {loading ? (
            <div style={{ color: 'var(--text-muted)' }}>Loading...</div>
          ) : (
            <textarea
              className="config-textarea"
              value={config}
              onChange={(e) => setConfig(e.target.value)}
              spellCheck="false"
            />
          )}
        </div>

        <div className="modal-footer">
          <button className="btn btn-stop btn-sm" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-start btn-sm" onClick={handleSave} disabled={loading}>
            Save
          </button>
        </div>
      </div>
    </div>
  )
}
