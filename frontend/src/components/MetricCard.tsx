import { useEffect, useRef, useState } from 'react';
import styles from './MetricCard.module.css';

type AccentKey = 'amber' | 'coral' | 'cyan' | 'red' | 'green' | 'yellow';
type Direction = 'export' | 'import' | 'charge' | 'discharge';

const ACCENT_CLASS: Record<AccentKey, string> = {
  amber:  styles.accentAmber,
  coral:  styles.accentCoral,
  cyan:   styles.accentCyan,
  red:    styles.accentRed,
  green:  styles.accentGreen,
  yellow: styles.accentYellow,
};

const DIRECTION_ARROWS: Record<Direction, { symbol: string; label: string; color: string }> = {
  export:    { symbol: '↑', label: 'Exporting',    color: 'var(--cyan)' },
  import:    { symbol: '↓', label: 'Importing',    color: 'var(--red)' },
  charge:    { symbol: '↑', label: 'Charging',     color: 'var(--green)' },
  discharge: { symbol: '↓', label: 'Discharging',  color: 'var(--yellow)' },
};

interface MetricCardProps {
  label: string;
  value: number;
  unit: string;
  accent: AccentKey;
  subtitle?: string;
  direction?: Direction;
}

export function MetricCard({ label, value, unit, accent, subtitle, direction }: MetricCardProps) {
  const [flash, setFlash] = useState(false);
  const prevValue = useRef(value);

  useEffect(() => {
    if (value !== prevValue.current) {
      prevValue.current = value;
      setFlash(true);
      const id = setTimeout(() => setFlash(false), 300);
      return () => clearTimeout(id);
    }
  }, [value]);

  const formatted = value.toLocaleString();
  const arrow = direction ? DIRECTION_ARROWS[direction] : null;

  return (
    <div className={`${styles.card} ${ACCENT_CLASS[accent]}`}>
      <span className={styles.label}>{label}</span>
      <div className={styles.valueRow}>
        <span className={`${styles.value}${flash ? ` ${styles.flash}` : ''}`}>
          {formatted}
        </span>
        <span className={styles.unit}>{unit}</span>
        {arrow && (
          <span
            role="img"
            aria-label={arrow.label}
            className={styles.direction}
            style={{ color: arrow.color }}
          >
            {arrow.symbol}
          </span>
        )}
      </div>
      {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
    </div>
  );
}
