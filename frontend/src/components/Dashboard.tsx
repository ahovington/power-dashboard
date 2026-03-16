import { useSSE } from '../hooks/useSSE';
import { usePowerStatus } from '../hooks/usePowerStatus';
import { usePowerHistory } from '../hooks/usePowerHistory';
import { PowerEvent } from '../types/power';
import { SSE_URL } from '../services/api';
import { Header } from './Header';
import { MetricCard } from './MetricCard';
import { EnergyFlowDiagram } from './EnergyFlowDiagram';
import { HistoryChart } from './HistoryChart';
import styles from './Dashboard.module.css';

interface DashboardProps {
  deviceId: string;
}

export function Dashboard({ deviceId }: DashboardProps) {
  const { latestEvent, connected } = useSSE<PowerEvent>(SSE_URL);
  const { status } = usePowerStatus(deviceId, latestEvent);
  const { readings, interval, setHistoryInterval, loading: historyLoading } = usePowerHistory(deviceId);

  const lastUpdated = latestEvent ? new Date(latestEvent.timestamp) : null;

  // Derive direction indicators
  const gridDirection = status && status.power_net >= 0 ? 'export' : 'import';

  return (
    <div className={styles.page}>
      <Header connected={connected} lastUpdated={lastUpdated} />

      <main className={styles.main}>
        <div className={styles.metricStrip}>
          <MetricCard
            label="SOLAR"
            value={status?.power_produced ?? 0}
            unit="W"
            accent="amber"
          />
          <MetricCard
            label="CONSUMING"
            value={status?.power_consumed ?? 0}
            unit="W"
            accent="coral"
          />
          <MetricCard
            label="GRID"
            value={status ? Math.abs(status.power_net) : 0}
            unit="W"
            accent={gridDirection === 'export' ? 'cyan' : 'red'}
            direction={gridDirection}
          />
        </div>

        <EnergyFlowDiagram
          solarW={status?.power_produced ?? 0}
          consumedW={status?.power_consumed ?? 0}
          netW={status?.power_net ?? 0}
          batteryW={0}
          batteryDirection="charging"
        />

        <HistoryChart
          readings={readings}
          interval={interval}
          onIntervalChange={setHistoryInterval}
          loading={historyLoading}
        />
      </main>
    </div>
  );
}
