import { PowerStatus, PowerReading, BatteryStatus, HistoryInterval } from '../types/power';

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export async function fetchPowerStatus(deviceId: string): Promise<PowerStatus> {
  const res = await fetch(`${BASE_URL}/api/v1/power/status?device_id=${deviceId}`);
  if (!res.ok) throw new Error(`fetchPowerStatus: HTTP ${res.status}`);
  return res.json();
}

export async function fetchPowerHistory(
  deviceId: string,
  interval: HistoryInterval,
  start: Date,
  end: Date
): Promise<PowerReading[]> {
  const params = new URLSearchParams({
    device_id: deviceId,
    interval,
    start: start.toISOString(),
    end: end.toISOString(),
  });
  const res = await fetch(`${BASE_URL}/api/v1/power/history?${params}`);
  if (!res.ok) throw new Error(`fetchPowerHistory: HTTP ${res.status}`);
  return res.json();
}

export async function fetchBatteryStatus(deviceId: string): Promise<BatteryStatus | null> {
  const res = await fetch(`${BASE_URL}/api/v1/power/battery?device_id=${deviceId}`);
  if (!res.ok) throw new Error(`fetchBatteryStatus: HTTP ${res.status}`);
  const data = await res.json();
  if (data.status === 'no data') return null;
  return data;
}

export const SSE_URL = `${BASE_URL}/api/v1/events`;
