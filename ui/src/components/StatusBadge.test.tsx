import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatusBadge } from './StatusBadge'

describe('StatusBadge', () => {
  it('renders running with green color', () => {
    render(<StatusBadge status="running" />)
    const badge = screen.getByText('running')
    expect(badge).toBeDefined()
    expect(badge.className).toContain('text-green-400')
    expect(badge.className).toContain('bg-green-400/10')
  })

  it('renders exited with red color', () => {
    render(<StatusBadge status="exited" />)
    const badge = screen.getByText('exited')
    expect(badge.className).toContain('text-red-400')
    expect(badge.className).toContain('bg-red-400/10')
  })

  it('renders unknown status with gray color', () => {
    render(<StatusBadge status="unknown" />)
    const badge = screen.getByText('unknown')
    expect(badge.className).toContain('text-gray-400')
    expect(badge.className).toContain('bg-gray-400/10')
  })

  it('renders paused with yellow color', () => {
    render(<StatusBadge status="paused" />)
    const badge = screen.getByText('paused')
    expect(badge.className).toContain('text-yellow-400')
  })
})
