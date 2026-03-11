import { useEffect, useRef } from 'preact/hooks'

export function Modal({ open, onClose, title, children }) {
  const overlayRef = useRef(null)

  useEffect(() => {
    if (!open) return
    const handleKey = (e) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onClose])

  if (!open) return null

  const handleOverlayClick = (e) => {
    if (e.target === overlayRef.current) onClose()
  }

  return (
    <div class="modal-overlay" ref={overlayRef} onClick={handleOverlayClick}>
      <div class="modal" role="dialog" aria-label={title}>
        <div class="modal-header">
          <h2 class="modal-title">{title}</h2>
          <button class="modal-close" onClick={onClose} aria-label="Close">&times;</button>
        </div>
        <div class="modal-body">
          {children}
        </div>
      </div>
    </div>
  )
}
