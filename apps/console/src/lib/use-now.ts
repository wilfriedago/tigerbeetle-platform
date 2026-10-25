import { useEffect, useState } from 'react'

/** The current time, refreshed every `intervalMs`, for countdowns and elapsed times. */
export function useNow(intervalMs = 1000): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = globalThis.setInterval(() => setNow(new Date()), intervalMs)
    return () => globalThis.clearInterval(timer)
  }, [intervalMs])
  return now
}
