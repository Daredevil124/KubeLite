import { useState, useCallback } from 'react'
import { useMetrics } from './hooks/useMetrics.js'
import MetricsRow    from './components/MetricsRow.jsx'
import TaskPanel     from './components/TaskPanel.jsx'
import ActivityLog, { makeEntry } from './components/ActivityLog.jsx'
import WorkerGrid    from './components/WorkerGrid.jsx'
import styles from './App.module.css'

export default function App() {
  const { data, online, rps, tps } = useMetrics()
  const [log, setLog] = useState([makeEntry('Dashboard ready — connecting to Master on :8080')])

  const addLog = useCallback((msg, type = 'info') => {
    setLog(prev => [makeEntry(msg, type), ...prev].slice(0, 30))
  }, [])

  const clearLog = useCallback(() => setLog([]), [])

  return (
    <div className={styles.app}>
      {/* ── HEADER ── */}
      <header className={styles.header}>
        <div className={styles.logo}>
          <span className={styles.logoMark}>K</span>
          <span className={styles.logoText}>KubeLite</span>
          <span className={styles.logoDivider}>/</span>
          <span className={styles.logoSub}>Distributed Task Engine</span>
        </div>

        <div className={styles.headerRight}>
          <div className={styles.pollNote}>polling every 2s</div>
          <div
            className={`${styles.statusChip} ${online ? styles.online : styles.offline}`}
          >
            <span className={styles.statusDot} />
            {online ? 'Master online' : 'Offline'}
          </div>
        </div>
      </header>

      {/* ── MAIN ── */}
      <main className={styles.main}>

        {/* Metrics */}
        <section>
          <div className={styles.sectionLabel}>Cluster Metrics</div>
          <MetricsRow data={data} rps={rps} tps={tps} />
        </section>

        {/* Control + Log */}
        <section>
          <div className={styles.sectionLabel}>Control</div>
          <div className={styles.midGrid}>
            <TaskPanel onLog={addLog} />
            <ActivityLog entries={log} onClear={clearLog} />
          </div>
        </section>

        {/* Workers */}
        <section>
          <div className={styles.sectionLabel}>
            Worker Nodes
            <span className={styles.workerCount}>
              {data?.worker_count ?? 0} active
            </span>
          </div>
          <div className="card" style={{ padding: '18px' }}>
            <WorkerGrid count={data?.worker_count ?? 0} />
          </div>
        </section>

      </main>
    </div>
  )
}
