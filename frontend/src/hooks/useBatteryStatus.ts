import { useState, useEffect } from 'react';
import { BatteryStatus, PowerEvent } from '../types/power';
import { fetchBatteryStatus } from '../services/api';

interface BatteryStatusState {
  battery: BatteryStatus | null;
  loading: boolean;
  error: string | null;
}

export function useBatteryStatus(
  deviceId: string,
  latestEvent?: PowerEvent | null
): BatteryStatusState {
  const [state, setState] = useState<BatteryStatusState>({
    battery: null,
    loading: true,
    error: null,
  });

  // Initial REST fetch
  useEffect(() => {
    fetchBatteryStatus(deviceId)
      .then(b => setState({ battery: b, loading: false, error: null }))
      .catch(err => setState({ battery: null, loading: false, error: String(err) }));
  }, [deviceId]);

  // Update power_flowing and direction from SSE without a full re-fetch.
  // Guard: battery_w == null means no battery data in this event — skip.
  useEffect(() => {
    if (latestEvent?.battery_w == null) return;
    setState(s => ({
      ...s,
      battery: s.battery ? {
        ...s.battery,
        power_flowing: latestEvent.battery_w!,
        power_direction: latestEvent.battery_direction ?? s.battery.power_direction,
      } : s.battery,
    }));
  }, [latestEvent]);

  return state;
}
