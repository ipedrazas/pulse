import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Header } from './Header'

describe('Header', () => {
  it('renders Pulse title', () => {
    render(<Header healthy={true} />)
    expect(screen.getByText('Pulse')).toBeDefined()
  })

  it('shows API Connected when healthy', () => {
    render(<Header healthy={true} />)
    expect(screen.getByText('API Connected')).toBeDefined()
  })

  it('shows API Disconnected when not healthy', () => {
    render(<Header healthy={false} />)
    expect(screen.getByText('API Disconnected')).toBeDefined()
  })

  it('shows green dot when healthy', () => {
    const { container } = render(<Header healthy={true} />)
    const dot = container.querySelector('span.bg-green-400')
    expect(dot).not.toBeNull()
  })

  it('shows red dot when not healthy', () => {
    const { container } = render(<Header healthy={false} />)
    const dot = container.querySelector('span.bg-red-400')
    expect(dot).not.toBeNull()
  })
})
