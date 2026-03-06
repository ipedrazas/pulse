import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('renders the message', () => {
    render(<EmptyState message="No containers found" />)
    expect(screen.getByText('No containers found')).toBeDefined()
  })

  it('renders custom message', () => {
    render(<EmptyState message="Nothing to see here" />)
    expect(screen.getByText('Nothing to see here')).toBeDefined()
  })
})
