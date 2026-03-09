import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { StatusDot } from './StatusDot'

describe('StatusDot', () => {
  it('renders green for online status', () => {
    const { container } = render(<StatusDot status="online" />)
    const dot = container.querySelector('span')
    expect(dot?.className).toContain('bg-green-400')
  })

  it('renders orange for offline status', () => {
    const { container } = render(<StatusDot status="offline" />)
    const dot = container.querySelector('span')
    expect(dot?.className).toContain('bg-orange-400')
  })

  it('renders red for lost status', () => {
    const { container } = render(<StatusDot status="lost" />)
    const dot = container.querySelector('span')
    expect(dot?.className).toContain('bg-red-400')
  })

  it('renders gray for unknown status', () => {
    const { container } = render(<StatusDot status="something" />)
    const dot = container.querySelector('span')
    expect(dot?.className).toContain('bg-gray-500')
  })

  it('sets title attribute', () => {
    const { container } = render(<StatusDot status="online" />)
    const dot = container.querySelector('span')
    expect(dot?.title).toBe('online')
  })
})
