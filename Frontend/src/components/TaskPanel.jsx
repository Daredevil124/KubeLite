import { useState } from 'react'
import { submitTask } from '../hooks/useMetrics.js'
import styles from './TaskPanel.module.css'

const TASKS = [
  {
    id:    'light',
    emoji: '◎',
    title: 'Light Task',
    desc:  'FindNthPrime(50000) · 1 core',
    color: 'var(--amber)',
    bg:    'var(--amber-2)',
  },
  {
    id:    'heavy',
    emoji: '◉',
    title: 'Heavy Task',
    desc:  'StressCPUAllCores(30s) · all cores',
    color: 'var(--red)',
    bg:    'var(--red-2)',
  },
]

export default function TaskPanel({ onLog }) {
  const [loading, setLoading] = useState(null)

  async function handleClick(type) {
    if (loading) return
    setLoading(type)
    try {
      await submitTask(type)
      onLog(`Queued: ${type === 'light' ? 'FindNthPrime(50000)' : 'StressCPUAllCores(30s)'}`, type)
    } catch {
      onLog('Submit failed — is Master running on :8080?', 'error')
    } finally {
      setLoading(null)
    }
  }

  return (
    <div className={`card ${styles.panel}`}>
      <div className={styles.header}>Submit Task</div>

      <div className={styles.list}>
        {TASKS.map(t => (
          <button
            key={t.id}
            className={styles.btn}
            style={{ '--c': t.color, '--bg': t.bg }}
            onClick={() => handleClick(t.id)}
            disabled={!!loading}
          >
            <span className={styles.btnEmoji}>{t.emoji}</span>
            <span className={styles.btnText}>
              <span className={styles.btnTitle}>{t.title}</span>
              <span className={styles.btnDesc}>{t.desc}</span>
            </span>
            <span className={styles.btnArrow}>
              {loading === t.id ? '…' : '→'}
            </span>
          </button>
        ))}
      </div>

      <p className={styles.hint}>
        Tasks are pushed to <code>task_queue</code> in Redis via{' '}
        <code>POST /request</code>. Workers pull and execute them.
      </p>
    </div>
  )
}
