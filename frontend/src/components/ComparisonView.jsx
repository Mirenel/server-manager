export default function ComparisonView({ processes, cpuHistoryRef, memHistoryRef, onClose }) {
  const renderSparkline = (data, color, max) => {
    if (!data || data.length === 0) {
      return <svg viewBox="0 0 100 30" style={{ width: '100%', height: '20px' }} />
    }

    const width = 100
    const height = 30
    const padding = 2
    const usableWidth = width - 2 * padding
    const usableHeight = height - 2 * padding

    const localMax = Math.max(...data, 1)
    const effectiveMax = max !== undefined ? Math.max(max, 1) : localMax

    const points = data.map((val, idx) => {
      const x = padding + (idx / (data.length - 1 || 1)) * usableWidth
      const y = height - padding - (val / effectiveMax) * usableHeight
      return `${x} ${y}`
    }).join(' ')

    const maxVal = Math.max(...data, 0)

    return (
      <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', height: '20px' }}>
        {points && (
          <polyline points={points} fill="none" stroke={color} strokeWidth="1.5" />
        )}
        <text x={width - 2} y={height - 2} fontSize="8" fill={color} textAnchor="end">
          {maxVal.toFixed(1)}
        </text>
      </svg>
    )
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="comparison-modal modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">Process Comparison</h2>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>

        <div className="modal-body">
          <div className="comparison-grid">
            {processes.map(proc => (
              <div key={proc.id} className="comparison-card">
                <div className="comparison-card-title">
                  <span
                    className="status-dot"
                    style={{
                      background:
                        proc.state === 'running'
                          ? 'var(--green)'
                          : proc.state === 'crashed'
                          ? 'var(--red)'
                          : 'var(--text-muted)',
                    }}
                  />
                  {proc.name}
                </div>

                <div className="comparison-sparkline">
                  <span className="comparison-sparkline-label">CPU</span>
                  {renderSparkline(cpuHistoryRef.current[proc.id], 'var(--green)', 100)}
                </div>

                <div className="comparison-sparkline">
                  <span className="comparison-sparkline-label">Memory</span>
                  {renderSparkline(memHistoryRef.current[proc.id], 'var(--blue)')}
                </div>

                <div className="comparison-stats">
                  {proc.state === 'running' ? (
                    <>
                      <div>CPU: {proc.cpu.toFixed(1)}%</div>
                      <div>RAM: {proc.memory_mb.toFixed(0)} MB</div>
                    </>
                  ) : (
                    <div>{proc.state}</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-stop btn-sm" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
