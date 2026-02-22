import { useEffect } from 'react'

const TOAST_DURATION = 4000

function ToastItem({ toast, onDismiss }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), TOAST_DURATION)
    return () => clearTimeout(timer)
  }, [toast.id, onDismiss])

  return (
    <div className="toast">
      <span className="toast-icon">⚠</span>
      <span className="toast-message">
        <span className="toast-name">{toast.name}</span> crashed
      </span>
      <button className="toast-dismiss" onClick={() => onDismiss(toast.id)}>×</button>
    </div>
  )
}

export default function Toast({ toasts, onDismiss }) {
  if (toasts.length === 0) return null

  return (
    <div className="toast-container">
      {toasts.map(t => (
        <ToastItem key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  )
}
