import { render, fireEvent } from '@testing-library/preact'
import { describe, it, expect, vi } from 'vitest'
import { Modal } from './modal.jsx'

describe('Modal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <Modal open={false} onClose={() => {}} title="Test">Content</Modal>
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders content when open', () => {
    const { getByText } = render(
      <Modal open={true} onClose={() => {}} title="Test Modal">
        <p>Modal content</p>
      </Modal>
    )
    expect(getByText('Test Modal')).toBeTruthy()
    expect(getByText('Modal content')).toBeTruthy()
  })

  it('calls onClose when close button clicked', () => {
    const onClose = vi.fn()
    const { getByLabelText } = render(
      <Modal open={true} onClose={onClose} title="Test">Content</Modal>
    )
    fireEvent.click(getByLabelText('Close'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when overlay clicked', () => {
    const onClose = vi.fn()
    const { container } = render(
      <Modal open={true} onClose={onClose} title="Test">Content</Modal>
    )
    fireEvent.click(container.querySelector('.modal-overlay'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not close when modal body clicked', () => {
    const onClose = vi.fn()
    const { getByText } = render(
      <Modal open={true} onClose={onClose} title="Test">
        <p>Click me</p>
      </Modal>
    )
    fireEvent.click(getByText('Click me'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('closes on Escape key', () => {
    const onClose = vi.fn()
    render(<Modal open={true} onClose={onClose} title="Test">Content</Modal>)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})
