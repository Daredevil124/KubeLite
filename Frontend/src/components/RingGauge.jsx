import { useState, useEffect, useRef } from 'react'
import styles from './RingGauge.module.css'

const R     = 30
const CIRC  = 2 * Math.PI * R   // ≈ 188.5

function getColor(pct) {
  if (pct > 80) return 'var(--red)'
  if (pct > 55) return 'var(--amber)'
  return 'var(--accent)'
}

export default function RingGauge({ value = 0, label }) {
  const pct    = Math.min(Math.max(value, 0), 100)
  const offset = CIRC - (pct / 100) * CIRC
  const color  = getColor(pct)

  // Animate the number smoothly
  const [display, setDisplay] = useState(0)
  const raf = useRef(null)
  useEffect(() => {
    const target = pct
    const start  = display
    const duration = 600
    const startTime = performance.now()
    const tick = (now) => {
      const t = Math.min((now - startTime) / duration, 1)
      setDisplay(start + (target - start) * t)
      if (t < 1) raf.current = requestAnimationFrame(tick)
    }
    raf.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf.current)
  }, [pct]) // eslint-disable-line

  return (
    <div className={styles.wrap}>
      <div className={styles.label}>{label}</div>
      <div className={styles.ring}>
        <svg width="76" height="76" viewBox="0 0 76 76" style={{ transform: 'rotate(-90deg)' }}>
          <circle
            cx="38" cy="38" r={R}
            fill="none" stroke="var(--border-hi)" strokeWidth="5"
          />
          <circle
            cx="38" cy="38" r={R}
            fill="none"
            stroke={color}
            strokeWidth="5"
            strokeLinecap="round"
            strokeDasharray={CIRC}
            strokeDashoffset={offset}
            style={{ transition: 'stroke-dashoffset 0.7s cubic-bezier(.4,0,.2,1), stroke 0.4s' }}
          />
        </svg>
        <div className={styles.value} style={{ color }}>
          {display.toFixed(0)}%
        </div>
      </div>
      <div className={styles.sub}>avg across workers</div>
    </div>
  )
}
