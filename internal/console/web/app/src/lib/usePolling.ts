import { useCallback, useEffect, useRef, useState } from 'react'

export interface PollResult<T> {
  data: T | null
  error: string | null
  loading: boolean
  refresh: () => void
}

// Polls an async fetcher on an interval. Skips overlapping calls and keeps the
// last good data on error so the UI can show a banner without going blank.
export function usePolling<T>(fetcher: () => Promise<T>, intervalMs: number): PollResult<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const inFlight = useRef(false)
  const fetcherRef = useRef(fetcher)

  // Keep the latest fetcher in a ref so `run` stays stable across renders.
  useEffect(() => {
    fetcherRef.current = fetcher
  }, [fetcher])

  const run = useCallback(async () => {
    if (inFlight.current) return
    inFlight.current = true
    try {
      const d = await fetcherRef.current()
      setData(d)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'request failed')
    } finally {
      inFlight.current = false
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void run()
    const id = window.setInterval(() => void run(), intervalMs)
    return () => window.clearInterval(id)
  }, [run, intervalMs])

  return { data, error, loading, refresh: run }
}
