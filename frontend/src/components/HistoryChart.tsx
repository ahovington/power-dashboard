import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { PowerReading, HistoryInterval } from '../types/power';
import styles from './HistoryChart.module.css';

const INTERVALS: HistoryInterval[] = ['hour', 'day', 'week', 'month'];

interface HistoryChartProps {
  readings: PowerReading[];
  interval: HistoryInterval;
  onIntervalChange: (i: HistoryInterval) => void;
  loading: boolean;
}

export function HistoryChart({ readings, interval, onIntervalChange, loading }: HistoryChartProps) {
  if (loading) {
    return (
      <div className={styles.container}>
        <div data-testid="chart-skeleton" className={styles.skeleton} />
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.sectionLabel}>History</span>
        <div className={styles.toggleGroup} role="group" aria-label="Chart interval">
          {INTERVALS.map(i => (
            <button
              key={i}
              className={`${styles.toggleBtn}${interval === i ? ` ${styles.active}` : ''}`}
              onClick={() => onIntervalChange(i)}
              aria-pressed={interval === i}
            >
              {i.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      <div className={styles.chartWrap}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={readings} margin={{ top: 4, right: 0, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id="solarGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--amber)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--amber)" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="consumeGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--coral)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--coral)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="reading_timestamp"
              hide
            />
            <YAxis hide />
            <Tooltip
              contentStyle={{
                background: 'var(--surface)',
                border: '1px solid var(--border)',
                borderRadius: 4,
                fontFamily: 'var(--font-condensed)',
                fontSize: 12,
                color: 'var(--text-primary)',
              }}
              formatter={(value: number, name: string) => [
                `${value.toLocaleString()} W`,
                name === 'power_produced' ? 'Solar' : 'Consuming',
              ]}
            />
            <Area
              type="monotone"
              dataKey="power_produced"
              stroke="var(--amber)"
              strokeWidth={1.5}
              fill="url(#solarGrad)"
              dot={false}
              activeDot={{ r: 3, fill: 'var(--amber)' }}
            />
            <Area
              type="monotone"
              dataKey="power_consumed"
              stroke="var(--coral)"
              strokeWidth={1.5}
              fill="url(#consumeGrad)"
              dot={false}
              activeDot={{ r: 3, fill: 'var(--coral)' }}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
