import { Modal } from './modal.jsx'

export function ConfirmDialog({ open, onClose, onConfirm, title, message, confirmText = 'Delete', danger = true }) {
  return (
    <Modal open={open} onClose={onClose} title={title || 'Confirm'}>
      <p class="confirm-message">{message}</p>
      <div class="confirm-actions">
        <button class="btn btn-secondary" onClick={onClose}>Cancel</button>
        <button class={`btn ${danger ? 'btn-danger' : 'btn-primary'}`} onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    </Modal>
  )
}
