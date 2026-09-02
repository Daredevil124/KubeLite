import { useRef, useState, useEffect, useCallback } from 'react'

const API = 'http://localhost:8080'

export function useMetrics() {
  const [data, setData]     = useState(null)
  const [online, setOnline] = useState(false)
  const [rps, setRps]       = useState(0)
  const [tps, setTps]       = useState(0)
  const prev = useRef({ req: 0, task: 0, time: Date.now() })

  const poll = useCallback(async () => {
    try {
      const res = await fetch(`${API}/metrics`, {
        signal: AbortSignal.timeout(3000),
      })
      if (!res.ok) throw new Error()
      const d = await res.json()

      const now     = Date.now()
      const elapsed = (now - prev.current.time) / 1000 || 1
      setRps(Math.max(0, (d.total_requests - prev.current.req)  / elapsed))
      setTps(Math.max(0, (d.total_tasks    - prev.current.task) / elapsed))
      prev.current = { req: d.total_requests, task: d.total_tasks, time: now }

      setData(d)
      setOnline(true)
    } catch {
      setOnline(false)
    }
  }, [])

  useEffect(() => {
    poll()
    const id = setInterval(poll, 2000)
    return () => clearInterval(id)
  }, [poll])

  return { data, online, rps, tps, poll }
}

export async function submitTask(type) {
  const payload = type === 'light'
    ? { type: 'light', n: 50000 }
    : { type: 'heavy', duration: 30 }

  const res = await fetch(`${API}/request`, {
    method:  'POST',
    headers: { 'Content-Type': 'application/json' },
    body:    JSON.stringify(payload),
    signal:  AbortSignal.timeout(5000),
  })
  if (!res.ok && res.status !== 202) throw new Error(`HTTP ${res.status}`)
}
