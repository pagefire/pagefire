import { render, fireEvent } from '@testing-library/preact'
import { describe, it, expect, vi } from 'vitest'
import { ConfirmDialog } from './confirm-dialog.jsx'

describe('ConfirmDialog', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <ConfirmDialog open={false} onClose={() => {}} onConfirm={() => {}} message="Sure?" />
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows message and buttons when open', () => {
    const { getByText } = render(
      <ConfirmDialog open={true} onClose={() => {}} onConfirm={() => {}} message="Delete this item?" />
    )
    expect(getByText('Delete this item?')).toBeTruthy()
    expect(getByText('Cancel')).toBeTruthy()
    expect(getByText('Delete')).toBeTruthy()
  })

  it('calls onClose when Cancel clicked', () => {
    const onClose = vi.fn()
    const { getByText } = render(
      <ConfirmDialog open={true} onClose={onClose} onConfirm={() => {}} message="Sure?" />
    )
    fireEvent.click(getByText('Cancel'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onConfirm when confirm button clicked', () => {
    const onConfirm = vi.fn()
    const { getByText } = render(
      <ConfirmDialog open={true} onClose={() => {}} onConfirm={onConfirm} message="Sure?" />
    )
    fireEvent.click(getByText('Delete'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('supports custom confirm text', () => {
    const { getByText } = render(
      <ConfirmDialog open={true} onClose={() => {}} onConfirm={() => {}} message="Sure?" confirmText="Remove" />
    )
    expect(getByText('Remove')).toBeTruthy()
  })
})
