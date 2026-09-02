import { useEffect, useRef, useState } from 'react'
import RingGauge from './RingGauge.jsx'
import styles from './MetricsRow.module.css'

function Stat({ label, value, unit, sub, color = 'var(--t1)' }) {
  const [display, setDisplay] = useState(value)
  const raf = useRef(null)
  const prev = useRef(value)

  useEffect(() => {
    const start = prev.current
    const target = value
    prev.current = value
    const startTime = performance.now()
    const tick = (now) => {
      const t = Math.min((now - startTime) / 500, 1)
      setDisplay(start + (target - start) * t)
      if (t < 1) raf.current = requestAnimationFrame(tick)
    }
    raf.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf.current)
  }, [value])

  const fmt = Number.isInteger(value)
    ? Math.round(display)
    : display.toFixed(1)

  return (
    <div className={styles.statCard}>
      <div className={styles.statLabel}>{label}</div>
      <div className={styles.statValue} style={{ color }}>
        <span className="mono">{fmt}</span>
        {unit && <span className={styles.statUnit}>{unit}</span>}
      </div>
      {sub && <div className={styles.statSub}>{sub}</div>}
    </div>
  )
}

export default function MetricsRow({ data, rps, tps }) {
  const cpu    = data?.cpu_percent    ?? 0
  const mem    = data?.memory_percent ?? 0
  const queue  = data?.queue_length   ?? 0
  const workers = data?.worker_count  ?? 0

  const queueColor = queue > 50 ? 'var(--red)' : queue > 10 ? 'var(--amber)' : 'var(--green)'

  return (
    <div className={styles.row}>
      {/* CPU ring */}
      <div className={`card ${styles.ringCard}`}>
        <RingGauge value={cpu} label="CPU" />
      </div>

      {/* Memory ring */}
      <div className={`card ${styles.ringCard}`}>
        <RingGauge value={mem} label="Memory" />
      </div>

      {/* Queue */}
      <Stat
        label="Queue"
        value={queue}
        unit=" tasks"
        sub="pending in Redis"
        color={queueColor}
      />

      {/* Workers */}
      <Stat
        label="Workers"
        value={workers}
        unit=" running"
        sub="label: role=worker"
        color="var(--green)"
      />

      {/* RPS */}
      <Stat
        label="RPS"
        value={rps}
        unit="/s"
        sub="requests per sec"
        color="var(--accent)"
      />

      {/* TPS */}
      <Stat
        label="TPS"
        value={tps}
        unit="/s"
        sub="tasks completed"
        color="var(--t2)"
      />
    </div>
  )
}
