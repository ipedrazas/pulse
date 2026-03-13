import { useEffect, useRef, useState, useCallback } from 'react'

export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs = 10000,
): { data: T | null; error: string | null; loading: boolean; refresh: () => void } {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const timerRef = useRef<ReturnType<typeof setInterval>>(null)
  const abortRef = useRef<AbortController | null>(null)
  const inflightRef = useRef(false)

  const doFetch = useCallback(async () => {
    // Skip if a fetch is already in flight (deduplication)
    if (inflightRef.current) return
    inflightRef.current = true

    // Abort any prior in-flight request
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    try {
      const result = await fetcher()
      // Only update state if this request wasn't aborted
      if (!controller.signal.aborted) {
        setData(result)
        setError(null)
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof Error ? e.message : 'Unknown error')
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false)
      }
      inflightRef.current = false
    }
  }, [fetcher])

  useEffect(() => {
    doFetch()
    timerRef.current = setInterval(doFetch, intervalMs)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
      abortRef.current?.abort()
    }
  }, [doFetch, intervalMs])

  return { data, error, loading, refresh: doFetch }
}
