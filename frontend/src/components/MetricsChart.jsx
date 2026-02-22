import { useEffect, useState } from 'react'

export default function MetricsChart({ processId }) {
  const [minutes, setMinutes] = useState(5)
  const [points, setPoints] = useState([])
  const [colors, setColors] = useState({
    green: '#3fb950',
    blue: '#58a6ff',
    border: '#30363d',
    textMuted: '#8b949e',
  })

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch(`/api/processes/${processId}/metrics?minutes=${minutes}`)
        if (res.ok) {
          const data = await res.json()
          setPoints(data.points || [])
        }
      } catch {
        // ignore fetch errors
      }
    }

    fetchMetrics()
    const interval = setInterval(fetchMetrics, 5000)
    return () => clearInterval(interval)
  }, [processId, minutes])

  // Get computed colors from CSS
  useEffect(() => {
    const getColors = () => {
      const style = getComputedStyle(document.documentElement)
      const green = style.getPropertyValue('--green').trim() || '#3fb950'
      const blue = style.getPropertyValue('--blue').trim() || '#58a6ff'
      const border = style.getPropertyValue('--border').trim() || '#30363d'
      const textMuted = style.getPropertyValue('--text-muted').trim() || '#8b949e'
      setColors({ green, blue, border, textMuted })
    }
    getColors()
  }, [document.documentElement.dataset.theme])

  const colorGreen = colors.green
  const colorBlue = colors.blue
  const colorBorder = colors.border
  const colorTextMuted = colors.textMuted

  // SVG rendering
  const viewBoxWidth = 400
  const viewBoxHeight = 80
  const padding = 10

  // Find max values for scaling
  const maxCpu = Math.max(...(points.map(p => p.cpu) || [0]), 100)
  const maxMem = Math.max(...(points.map(p => p.mem_mb) || [0]), 1)

  const scaleX = (idx) => {
    const range = points.length - 1 || 1
    return padding + (idx / range) * (viewBoxWidth - 2 * padding)
  }

  const scaleCpuY = (cpu) => {
    return viewBoxHeight - padding - (cpu / 100) * (viewBoxHeight - 2 * padding)
  }

  const scaleMemY = (mem) => {
    return viewBoxHeight - padding - (mem / maxMem) * (viewBoxHeight - 2 * padding)
  }

  // CPU path
  const cpuPath = points.length > 0
    ? points.map((p, idx) => `${scaleX(idx)} ${scaleCpuY(p.cpu)}`).join(' ')
    : ''

  // Memory path
  const memPath = points.length > 0
    ? points.map((p, idx) => `${scaleX(idx)} ${scaleMemY(p.mem_mb)}`).join(' ')
    : ''

  const windowOptions = [
    { label: '1m', value: 1 },
    { label: '5m', value: 5 },
    { label: '15m', value: 15 },
    { label: '30m', value: 30 },
    { label: '60m', value: 60 },
  ]

  return (
    <div className="metrics-chart">
      <div className="metrics-header">
        <span className="metrics-title">Metrics History</span>
        <div className="metrics-window">
          {windowOptions.map(opt => (
            <button
              key={opt.value}
              className={`metrics-btn ${minutes === opt.value ? 'active' : ''}`}
              onClick={() => setMinutes(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {/* CPU Chart */}
      <div>
        <div style={{ fontSize: '11px', fontWeight: '600', color: colorGreen, marginBottom: '4px' }}>
          CPU (0-100%)
        </div>
        <svg className="metrics-svg" viewBox={`0 0 ${viewBoxWidth} ${viewBoxHeight}`}>
          {/* Gridlines */}
          <line x1={padding} y1={padding} x2={viewBoxWidth - padding} y2={padding} stroke={colorBorder} strokeWidth="1" opacity="0.5" />
          <line x1={padding} y1={viewBoxHeight - padding} x2={viewBoxWidth - padding} y2={viewBoxHeight - padding} stroke={colorBorder} strokeWidth="1" opacity="0.5" />

          {/* Y-axis labels */}
          <text x={padding - 5} y={padding + 3} fontSize="8" fill={colorTextMuted} textAnchor="end">100%</text>
          <text x={padding - 5} y={viewBoxHeight - padding + 3} fontSize="8" fill={colorTextMuted} textAnchor="end">0%</text>

          {/* CPU line */}
          {cpuPath && (
            <polyline
              points={cpuPath}
              fill="none"
              stroke={colorGreen}
              strokeWidth="2"
              opacity="0.75"
            />
          )}
        </svg>
      </div>

      {/* Memory Chart */}
      <div>
        <div style={{ fontSize: '11px', fontWeight: '600', color: colorBlue, marginBottom: '4px' }}>
          Memory ({maxMem.toFixed(0)} MB max)
        </div>
        <svg className="metrics-svg" viewBox={`0 0 ${viewBoxWidth} ${viewBoxHeight}`}>
          {/* Gridlines */}
          <line x1={padding} y1={padding} x2={viewBoxWidth - padding} y2={padding} stroke={colorBorder} strokeWidth="1" opacity="0.5" />
          <line x1={padding} y1={viewBoxHeight - padding} x2={viewBoxWidth - padding} y2={viewBoxHeight - padding} stroke={colorBorder} strokeWidth="1" opacity="0.5" />

          {/* Y-axis labels */}
          <text x={padding - 5} y={padding + 3} fontSize="8" fill={colorTextMuted} textAnchor="end">{maxMem.toFixed(0)}</text>
          <text x={padding - 5} y={viewBoxHeight - padding + 3} fontSize="8" fill={colorTextMuted} textAnchor="end">0</text>

          {/* Memory line */}
          {memPath && (
            <polyline
              points={memPath}
              fill="none"
              stroke={colorBlue}
              strokeWidth="2"
              opacity="0.75"
            />
          )}
        </svg>
      </div>
    </div>
  )
}
