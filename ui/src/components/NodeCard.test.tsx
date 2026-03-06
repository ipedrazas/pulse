import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { NodeCard } from './NodeCard'
import type { Agent } from '../types'

const mockAgent: Agent = {
  name: 'node-1',
  status: 'online',
  version: '0.1.0',
  last_seen: '2026-03-06T12:00:00Z',
  container_count: 5,
}

describe('NodeCard', () => {
  it('renders node name', () => {
    render(<NodeCard agent={mockAgent} selected={false} onClick={() => {}} />)
    expect(screen.getByText('node-1')).toBeDefined()
  })

  it('renders version', () => {
    render(<NodeCard agent={mockAgent} selected={false} onClick={() => {}} />)
    expect(screen.getByText('v0.1.0')).toBeDefined()
  })

  it('renders container count', () => {
    render(<NodeCard agent={mockAgent} selected={false} onClick={() => {}} />)
    expect(screen.getByText('5 containers')).toBeDefined()
  })

  it('applies selected style', () => {
    const { container } = render(<NodeCard agent={mockAgent} selected={true} onClick={() => {}} />)
    const button = container.querySelector('button')
    expect(button?.className).toContain('border-blue-500')
  })

  it('applies unselected style', () => {
    const { container } = render(<NodeCard agent={mockAgent} selected={false} onClick={() => {}} />)
    const button = container.querySelector('button')
    expect(button?.className).toContain('border-gray-800')
  })

  it('calls onClick when clicked', () => {
    const onClick = vi.fn()
    render(<NodeCard agent={mockAgent} selected={false} onClick={onClick} />)
    fireEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
