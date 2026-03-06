import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { usePolling } from './usePolling'

describe('usePolling', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('calls fetcher on mount and sets data', async () => {
    const fetcher = vi.fn().mockResolvedValue({ nodes: [] })
    const { result } = renderHook(() => usePolling(fetcher, 5000))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.data).toEqual({ nodes: [] })
    expect(result.current.error).toBeNull()
    expect(fetcher).toHaveBeenCalled()
  })

  it('sets error on failure', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('network error'))
    const { result } = renderHook(() => usePolling(fetcher, 5000))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe('network error')
  })

  it('polls on interval', async () => {
    const fetcher = vi.fn().mockResolvedValue('data')
    renderHook(() => usePolling(fetcher, 1000))

    await waitFor(() => {
      expect(fetcher).toHaveBeenCalled()
    })

    const initialCount = fetcher.mock.calls.length

    // Advance by one interval
    await act(async () => {
      vi.advanceTimersByTime(1000)
    })

    await waitFor(() => {
      expect(fetcher.mock.calls.length).toBeGreaterThan(initialCount)
    })
  })

  it('cleans up on unmount', async () => {
    const fetcher = vi.fn().mockResolvedValue('data')
    const { unmount } = renderHook(() => usePolling(fetcher, 1000))

    await waitFor(() => {
      expect(fetcher).toHaveBeenCalled()
    })

    const callCount = fetcher.mock.calls.length
    unmount()

    // Advance timers — fetcher should NOT be called again
    vi.advanceTimersByTime(5000)
    expect(fetcher).toHaveBeenCalledTimes(callCount)
  })

  it('refresh triggers immediate fetch', async () => {
    const fetcher = vi.fn().mockResolvedValue('data')
    const { result } = renderHook(() => usePolling(fetcher, 60000))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    const countBefore = fetcher.mock.calls.length

    await act(async () => {
      result.current.refresh()
    })

    await waitFor(() => {
      expect(fetcher.mock.calls.length).toBeGreaterThan(countBefore)
    })
  })
})
