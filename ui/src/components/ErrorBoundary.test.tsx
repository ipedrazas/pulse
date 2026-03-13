import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ErrorBoundary } from './ErrorBoundary'

function ThrowingComponent({ message }: { message: string }) {
  throw new Error(message)
}

function GoodComponent() {
  return <p>Everything is fine</p>
}

describe('ErrorBoundary', () => {
  // Suppress React error boundary console.error noise in tests
  const originalError = console.error
  beforeEach(() => {
    console.error = vi.fn()
  })
  afterEach(() => {
    console.error = originalError
  })

  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <GoodComponent />
      </ErrorBoundary>,
    )
    expect(screen.getByText('Everything is fine')).toBeDefined()
  })

  it('renders fallback UI when a child throws', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent message="test crash" />
      </ErrorBoundary>,
    )
    expect(screen.getByText('Something went wrong')).toBeDefined()
    expect(screen.getByText('test crash')).toBeDefined()
  })

  it('renders custom fallback when provided', () => {
    render(
      <ErrorBoundary fallback={<p>Custom fallback</p>}>
        <ThrowingComponent message="boom" />
      </ErrorBoundary>,
    )
    expect(screen.getByText('Custom fallback')).toBeDefined()
  })

  it('recovers when "Try again" is clicked', async () => {
    const user = userEvent.setup()

    // We need a component that throws on first render but not on re-render
    let shouldThrow = true
    function ConditionalThrower() {
      if (shouldThrow) throw new Error('initial error')
      return <p>Recovered</p>
    }

    render(
      <ErrorBoundary>
        <ConditionalThrower />
      </ErrorBoundary>,
    )

    expect(screen.getByText('Something went wrong')).toBeDefined()

    // Fix the component before clicking retry
    shouldThrow = false
    await user.click(screen.getByText('Try again'))

    expect(screen.getByText('Recovered')).toBeDefined()
  })
})
