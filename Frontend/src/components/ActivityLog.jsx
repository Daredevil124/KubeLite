import styles from './ActivityLog.module.css'

const PIP = {
  light: 'var(--amber)',
  heavy: 'var(--red)',
  info:  'var(--accent)',
  error: 'var(--red)',
}

function ts() {
  return new Date().toLocaleTimeString('en-US', { hour12: false })
}

export function makeEntry(msg, type = 'info') {
  return { id: Date.now() + Math.random(), msg, type, time: ts() }
}

export default function ActivityLog({ entries, onClear }) {
  return (
    <div className={`card ${styles.panel}`}>
      <div className={styles.header}>
        <span>Activity</span>
        <button className={styles.clearBtn} onClick={onClear}>Clear</button>
      </div>

      <div className={styles.scroll}>
        {entries.length === 0 && (
          <div className={styles.empty}>No activity yet</div>
        )}
        {entries.map(e => (
          <div key={e.id} className={styles.row}>
            <span className={styles.pip} style={{ background: PIP[e.type] }} />
            <span className={styles.msg}>{e.msg}</span>
            <span className={`${styles.time} mono`}>{e.time}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
