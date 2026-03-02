import { useCallback, useEffect, useRef, useState } from "react";

interface UsePollingResult<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  lastUpdated: Date | null;
}

export function usePolling<T>(fetcher: () => Promise<T>, intervalMs: number): UsePollingResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const mountedRef = useRef(true);

  const poll = useCallback(async () => {
    try {
      const result = await fetcher();
      if (mountedRef.current) {
        setData(result);
        setError(null);
        setLastUpdated(new Date());
        setLoading(false);
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof Error ? err.message : "Unknown error");
        setLoading(false);
      }
    }
  }, [fetcher]);

  useEffect(() => {
    mountedRef.current = true;
    poll();
    const id = setInterval(poll, intervalMs);
    return () => {
      mountedRef.current = false;
      clearInterval(id);
    };
  }, [poll, intervalMs]);

  return { data, error, loading, lastUpdated };
}
