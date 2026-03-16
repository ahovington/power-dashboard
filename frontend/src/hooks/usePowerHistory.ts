import { useState, useEffect, useCallback } from 'react';
import { PowerReading, HistoryInterval } from '../types/power';
import { fetchPowerHistory } from '../services/api';

interface PowerHistoryState {
  readings: PowerReading[];
  loading: boolean;
  error: string | null;
  interval: HistoryInterval;
  setHistoryInterval: (i: HistoryInterval) => void;
}

// Returns the correct start date for each interval.
// Uses setDate/setMonth — NOT setHours — to avoid subtracting hours instead of days.
function defaultRange(interval: HistoryInterval): { start: Date; end: Date } {
  const end = new Date();
  const start = new Date(end);
  switch (interval) {
    case 'hour':  start.setHours(start.getHours() - 24); break;
    case 'day':   start.setDate(start.getDate() - 7); break;
    case 'week':  start.setDate(start.getDate() - 28); break;
    case 'month': start.setMonth(start.getMonth() - 12); break;
  }
  return { start, end };
}

export function usePowerHistory(deviceId: string): PowerHistoryState {
  const [interval, setIntervalState] = useState<HistoryInterval>('hour');
  const [state, setState] = useState<Omit<PowerHistoryState, 'interval' | 'setHistoryInterval'>>({
    readings: [],
    loading: true,
    error: null,
  });

  useEffect(() => {
    setState(s => ({ ...s, loading: true }));
    const { start, end } = defaultRange(interval);
    fetchPowerHistory(deviceId, interval, start, end)
      .then(readings => setState({ readings, loading: false, error: null }))
      .catch(err => setState({ readings: [], loading: false, error: String(err) }));
  }, [deviceId, interval]);

  const setHistoryInterval = useCallback((i: HistoryInterval) => setIntervalState(i), []);

  return { ...state, interval, setHistoryInterval };
}
