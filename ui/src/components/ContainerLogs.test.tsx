import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ContainerLogs } from './ContainerLogs'
import * as client from '../api/client'

vi.mock('../api/client', () => ({
  requestContainerLogs: vi.fn(),
  getCommandResult: vi.fn(),
}))

describe('ContainerLogs', () => {
  const onClose = vi.fn()

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.clearAllMocks()
    // jsdom doesn't implement scrollIntoView
    Element.prototype.scrollIntoView = vi.fn()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows loading state initially', () => {
    vi.mocked(client.requestContainerLogs).mockImplementation(
      () => new Promise(() => {}), // never resolves
    )

    render(<ContainerLogs containerId="c1" onClose={onClose} />)
    expect(screen.getByText('Loading logs...')).toBeDefined()
  })

  it('renders log lines on success', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-1',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'log-1',
      status: 'completed',
      result: 'line one\nline two\nline three',
    })

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    // Flush the requestContainerLogs promise, then advance past 500ms setTimeout + poll
    await act(async () => {
      await vi.advanceTimersByTimeAsync(600)
    })

    expect(screen.getByText('line one')).toBeDefined()
    expect(screen.getByText('line two')).toBeDefined()
    expect(screen.getByText('line three')).toBeDefined()
  })

  it('shows (no output) when result is empty', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-2',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'log-2',
      status: 'completed',
      result: '',
    })

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(600)
    })

    expect(screen.getByText('(no output)')).toBeDefined()
  })

  it('shows error when requestContainerLogs fails', async () => {
    vi.mocked(client.requestContainerLogs).mockRejectedValue(new Error('request failed'))

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    await act(async () => {
      vi.advanceTimersByTime(100)
    })

    expect(screen.getByText('request failed')).toBeDefined()
  })

  it('shows error when poll fails', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-3',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockRejectedValue(new Error('poll error'))

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(600)
    })

    expect(screen.getByText('poll error')).toBeDefined()
  })

  it('polls again when status is pending', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-4',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult)
      .mockResolvedValueOnce({ command_id: 'log-4', status: 'pending' })
      .mockResolvedValueOnce({
        command_id: 'log-4',
        status: 'completed',
        result: 'final output',
      })

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    // First poll at 500ms — returns pending
    await act(async () => {
      await vi.advanceTimersByTimeAsync(600)
    })

    // Second poll at 500 + 1000ms — returns completed
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })

    expect(screen.getByText('final output')).toBeDefined()
    expect(client.getCommandResult).toHaveBeenCalledTimes(2)
  })

  it('calls onClose when Close button is clicked', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.requestContainerLogs).mockImplementation(() => new Promise(() => {}))

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    await user.click(screen.getByText('Close'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not update state after unmount', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-5',
      status: 'pending',
    })
    // Slow poll — will resolve after unmount
    vi.mocked(client.getCommandResult).mockImplementation(() => new Promise(() => {}))

    const { unmount } = render(<ContainerLogs containerId="c1" onClose={onClose} />)

    // Unmount before poll completes
    unmount()

    // Advance timers — should not throw
    await act(async () => {
      vi.advanceTimersByTime(5000)
    })
  })

  it('renders failed status result as log lines', async () => {
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-6',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'log-6',
      status: 'failed',
      result: 'error output here',
    })

    render(<ContainerLogs containerId="c1" onClose={onClose} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(600)
    })

    // Failed status should still render the result as log lines
    expect(screen.getByText('error output here')).toBeDefined()
  })

  it('passes containerId to requestContainerLogs', async () => {
    vi.mocked(client.requestContainerLogs).mockImplementation(() => new Promise(() => {}))

    render(<ContainerLogs containerId="my-container-xyz" onClose={onClose} />)

    expect(client.requestContainerLogs).toHaveBeenCalledWith('my-container-xyz', 200)
  })
})
