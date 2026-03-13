import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ContainerDetail } from './ContainerDetail'
import type { Container } from '../types'
import * as client from '../api/client'

// Mock the API client module
vi.mock('../api/client', () => ({
  stopContainer: vi.fn(),
  restartContainer: vi.fn(),
  pullContainerImage: vi.fn(),
  getCommandResult: vi.fn(),
  requestContainerLogs: vi.fn(),
}))

const runningContainer: Container = {
  container_id: 'abc123',
  agent_name: 'node-1',
  name: 'web',
  image: 'nginx:latest',
  status: 'running',
  env_vars: { PORT: '8080', DB_HOST: 'localhost' },
  mounts: ['/data:/app/data'],
  labels: { app: 'web' },
  ports: [{ host_ip: '0.0.0.0', host_port: 80, container_port: 8080, protocol: 'tcp' }],
  compose_project: '',
  command: 'nginx -g daemon off;',
  uptime_seconds: 3600,
  created_at: '2026-03-01T00:00:00Z',
}

const exitedContainer: Container = {
  ...runningContainer,
  status: 'exited',
}

const composeContainer: Container = {
  ...runningContainer,
  compose_project: 'myapp',
  labels: { 'com.docker.compose.project': 'myapp', app: 'web' },
}

describe('ContainerDetail', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // --- Rendering ---

  it('renders container info', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('abc123')).toBeDefined()
    expect(screen.getByText('nginx -g daemon off;')).toBeDefined()
  })

  it('renders environment variables', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('PORT:')).toBeDefined()
    expect(screen.getByText('8080')).toBeDefined()
    expect(screen.getByText('DB_HOST:')).toBeDefined()
  })

  it('renders ports', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('0.0.0.0:80 -> 8080')).toBeDefined()
  })

  it('renders mounts', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('/data:/app/data')).toBeDefined()
  })

  it('renders labels', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('app:')).toBeDefined()
  })

  it('shows "None" when env vars are empty', () => {
    const c = { ...runningContainer, env_vars: {} }
    render(<ContainerDetail container={c} />)
    // Multiple "None" sections possible
    const nones = screen.getAllByText('None')
    expect(nones.length).toBeGreaterThan(0)
  })

  // --- Action buttons for running containers ---

  it('shows Stop and Restart for running containers', () => {
    render(<ContainerDetail container={runningContainer} />)
    expect(screen.getByText('Stop')).toBeDefined()
    expect(screen.getByText('Restart')).toBeDefined()
    expect(screen.getByText('Pull Image')).toBeDefined()
  })

  it('hides Stop and Restart for exited containers', () => {
    render(<ContainerDetail container={exitedContainer} />)
    expect(screen.queryByText('Stop')).toBeNull()
    expect(screen.queryByText('Restart')).toBeNull()
    expect(screen.getByText('Pull Image')).toBeDefined()
  })

  it('shows compose labels for compose containers', () => {
    render(<ContainerDetail container={composeContainer} />)
    expect(screen.getByText('Redeploy Stack')).toBeDefined()
    expect(screen.getByText('Pull & Redeploy')).toBeDefined()
  })

  // --- Action flow: pending → success ---

  it('transitions stop action through pending to success', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.stopContainer).mockResolvedValue({
      command_id: 'cmd-1',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'cmd-1',
      status: 'completed',
    })

    render(<ContainerDetail container={runningContainer} />)

    // Click stop
    await user.click(screen.getByText('Stop'))

    // Should show pending
    expect(screen.getByText('...')).toBeDefined()

    // Advance past the 500ms initial delay + poll
    await act(async () => {
      vi.advanceTimersByTime(600)
    })

    // Should show success
    expect(screen.getByText('Done')).toBeDefined()
    expect(client.stopContainer).toHaveBeenCalledWith('abc123')
    expect(client.getCommandResult).toHaveBeenCalledWith('cmd-1')
  })

  // --- Action flow: pending → error ---

  it('transitions action to error on command failure', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.stopContainer).mockResolvedValue({
      command_id: 'cmd-2',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'cmd-2',
      status: 'failed',
      result: 'container not found',
    })

    render(<ContainerDetail container={runningContainer} />)

    await user.click(screen.getByText('Stop'))

    await act(async () => {
      vi.advanceTimersByTime(600)
    })

    expect(screen.getByText('Failed')).toBeDefined()
    expect(screen.getByText('container not found')).toBeDefined()
  })

  // --- Action flow: immediate error ---

  it('shows error when action API call fails', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.stopContainer).mockRejectedValue(new Error('network error'))

    render(<ContainerDetail container={runningContainer} />)

    await user.click(screen.getByText('Stop'))

    // Allow promise to settle
    await act(async () => {
      vi.advanceTimersByTime(0)
    })

    expect(screen.getByText('Failed')).toBeDefined()
    expect(screen.getByText('network error')).toBeDefined()
  })

  // --- Action flow: poll error ---

  it('shows error when poll fails', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.pullContainerImage).mockResolvedValue({
      command_id: 'cmd-3',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockRejectedValue(new Error('poll timeout'))

    render(<ContainerDetail container={runningContainer} />)

    await user.click(screen.getByText('Pull Image'))

    await act(async () => {
      vi.advanceTimersByTime(600)
    })

    expect(screen.getByText('Failed')).toBeDefined()
    expect(screen.getByText('poll timeout')).toBeDefined()
  })

  // --- Action flow: pending poll retry ---

  it('polls again when status is still pending', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.restartContainer).mockResolvedValue({
      command_id: 'cmd-4',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult)
      .mockResolvedValueOnce({ command_id: 'cmd-4', status: 'pending' })
      .mockResolvedValueOnce({ command_id: 'cmd-4', status: 'completed' })

    render(<ContainerDetail container={runningContainer} />)

    await user.click(screen.getByText('Restart'))

    // First poll after 500ms — returns pending
    await act(async () => {
      vi.advanceTimersByTime(600)
    })
    expect(screen.getByText('...')).toBeDefined()

    // Second poll after another 1000ms — returns completed
    await act(async () => {
      vi.advanceTimersByTime(1100)
    })
    expect(screen.getByText('Done')).toBeDefined()
    expect(client.getCommandResult).toHaveBeenCalledTimes(2)
  })

  // --- Logs toggle ---

  it('toggles log view on button click', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    // Mock the log request
    vi.mocked(client.requestContainerLogs).mockResolvedValue({
      command_id: 'log-1',
      status: 'pending',
    })
    vi.mocked(client.getCommandResult).mockResolvedValue({
      command_id: 'log-1',
      status: 'completed',
      result: 'log line 1\nlog line 2',
    })

    render(<ContainerDetail container={runningContainer} />)

    // Click View Logs
    await user.click(screen.getByText('View Logs'))
    expect(screen.getByText('Hide Logs')).toBeDefined()
    expect(screen.getByText('Logs')).toBeDefined()

    // Click Hide Logs
    await user.click(screen.getByText('Hide Logs'))
    expect(screen.getByText('View Logs')).toBeDefined()
  })

  // --- Cleanup on unmount ---

  it('does not error when unmounted during pending action', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(client.stopContainer).mockResolvedValue({
      command_id: 'cmd-cleanup',
      status: 'pending',
    })
    // Never resolve the poll — simulate slow server
    vi.mocked(client.getCommandResult).mockImplementation(
      () => new Promise(() => {}), // never resolves
    )

    const { unmount } = render(<ContainerDetail container={runningContainer} />)

    await user.click(screen.getByText('Stop'))

    // Unmount while action is pending — should not throw
    unmount()

    // Advance timers after unmount — timers should be cleaned up
    await act(async () => {
      vi.advanceTimersByTime(5000)
    })
  })
})
