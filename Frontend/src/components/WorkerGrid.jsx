import styles from './WorkerGrid.module.css'

export default function WorkerGrid({ count = 0 }) {
  if (count === 0) {
    return (
      <div className={styles.empty}>
        <span className={styles.emptyIcon}>▣</span>
        <span>No containers with <code>role=worker</code> running</span>
        <code className={styles.cmd}>docker run -d --label role=worker kubelite-worker</code>
      </div>
    )
  }

  return (
    <div className={styles.grid}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className={styles.node}>
          <div className={styles.nodeHead}>
            <span className={styles.led} />
            <span className={`${styles.nodeId} mono`}>worker-{i + 1}</span>
          </div>
          <div className={styles.nodeRows}>
            <div className={styles.nodeRow}>
              <span className={styles.key}>status</span>
              <span className={`${styles.val} mono`} style={{ color: 'var(--green)' }}>running</span>
            </div>
            <div className={styles.nodeRow}>
              <span className={styles.key}>label</span>
              <span className={`${styles.val} mono`}>role=worker</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
