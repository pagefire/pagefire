import { render, fireEvent } from '@testing-library/preact'
import { describe, it, expect, vi } from 'vitest'
import { TextInput, SelectInput } from './form-field.jsx'

describe('TextInput', () => {
  it('renders label and input', () => {
    const { getByText, container } = render(
      <TextInput label="Name" value="" onInput={() => {}} />
    )
    expect(getByText('Name')).toBeTruthy()
    expect(container.querySelector('input[type="text"]')).toBeTruthy()
  })

  it('shows error state', () => {
    const { getByText, container } = render(
      <TextInput label="Email" value="" error="Required" onInput={() => {}} />
    )
    expect(getByText('Required')).toBeTruthy()
    expect(container.querySelector('.form-control-error')).toBeTruthy()
  })

  it('calls onInput when typing', () => {
    const onInput = vi.fn()
    const { container } = render(
      <TextInput label="Name" value="" onInput={onInput} />
    )
    fireEvent.input(container.querySelector('input'), { target: { value: 'Jane' } })
    expect(onInput).toHaveBeenCalled()
  })
})

describe('SelectInput', () => {
  const options = [
    { value: 'a', label: 'Option A' },
    { value: 'b', label: 'Option B' },
  ]

  it('renders options', () => {
    const { getByText } = render(
      <SelectInput label="Choose" value="" options={options} onChange={() => {}} />
    )
    expect(getByText('Option A')).toBeTruthy()
    expect(getByText('Option B')).toBeTruthy()
  })

  it('renders placeholder', () => {
    const { getByText } = render(
      <SelectInput label="Choose" value="" options={options} placeholder="Pick one" onChange={() => {}} />
    )
    expect(getByText('Pick one')).toBeTruthy()
  })
})
