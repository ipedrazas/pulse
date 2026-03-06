import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SearchBar } from './SearchBar'

describe('SearchBar', () => {
  it('renders with placeholder', () => {
    render(<SearchBar value="" onChange={() => {}} placeholder="Find containers..." />)
    expect(screen.getByPlaceholderText('Find containers...')).toBeDefined()
  })

  it('renders with default placeholder', () => {
    render(<SearchBar value="" onChange={() => {}} />)
    expect(screen.getByPlaceholderText('Search...')).toBeDefined()
  })

  it('calls onChange when typing', () => {
    const onChange = vi.fn()
    render(<SearchBar value="" onChange={onChange} />)
    const input = screen.getByPlaceholderText('Search...')
    fireEvent.change(input, { target: { value: 'nginx' } })
    expect(onChange).toHaveBeenCalledWith('nginx')
  })

  it('displays current value', () => {
    render(<SearchBar value="hello" onChange={() => {}} />)
    const input = screen.getByDisplayValue('hello')
    expect(input).toBeDefined()
  })
})
