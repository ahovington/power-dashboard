import { useState, useEffect } from 'react';
import { PowerStatus, PowerEvent } from '../types/power';
import { fetchPowerStatus } from '../services/api';

interface PowerStatusState {
  status: PowerStatus | null;
  loading: boolean;
  error: string | null;
}

export function usePowerStatus(
  deviceId: string,
  liveEvent?: PowerEvent | null
): PowerStatusState {
  const [state, setState] = useState<PowerStatusState>({
    status: null,
    loading: true,
    error: null,
  });

  // Initial REST fetch
  useEffect(() => {
    fetchPowerStatus(deviceId)
      .then(status => setState({ status, loading: false, error: null }))
      .catch(err => setState({ status: null, loading: false, error: String(err) }));
  }, [deviceId]);

  // Merge live SSE event without re-fetching
  useEffect(() => {
    if (!liveEvent) return;
    setState(s => ({
      ...s,
      status: s.status
        ? { ...s.status, ...liveEvent, timestamp: liveEvent.timestamp }
        : null,
    }));
  }, [liveEvent]);

  return state;
}
