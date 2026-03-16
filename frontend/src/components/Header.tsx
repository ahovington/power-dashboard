import styles from './Header.module.css';

interface HeaderProps {
  connected: boolean;
  lastUpdated: Date | null;
}

export function Header({ connected, lastUpdated }: HeaderProps) {
  const timeStr = lastUpdated
    ? lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : null;

  return (
    <header className={styles.header}>
      <div className={styles.brand}>
        <div className={styles.appName}>
          Power <span>/</span> Monitor
        </div>
        <div className={styles.deviceLabel}>Home System</div>
      </div>

      <div className={styles.status}>
        {timeStr && (
          <span className={styles.timestamp}>{timeStr}</span>
        )}
        <div className={styles.liveGroup}>
          <span className={`${styles.dot}${connected ? '' : ` ${styles.dotOffline}`}`} />
          <span className={`${styles.liveText} ${connected ? styles.online : styles.offline}`}>
            {connected ? 'LIVE' : 'RECONNECTING'}
          </span>
        </div>
      </div>
    </header>
  );
}
