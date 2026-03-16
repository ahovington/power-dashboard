import { http, HttpResponse } from 'msw';
import { PowerStatus, PowerReading } from '../types/power';

export const MOCK_DEVICE_ID = 'test-device-uuid';

export const MOCK_STATUS: PowerStatus = {
  device_id: MOCK_DEVICE_ID,
  timestamp: '2024-06-01T12:00:00.000Z',
  power_produced: 5234,
  power_consumed: 3100,
  power_net: 2134,
};

export const MOCK_HISTORY: PowerReading[] = [
  { reading_timestamp: '2024-01-01T10:00:00Z', power_produced: 4000, power_consumed: 2800 },
  { reading_timestamp: '2024-01-01T11:00:00Z', power_produced: 5000, power_consumed: 3100 },
  { reading_timestamp: '2024-01-01T12:00:00Z', power_produced: 5234, power_consumed: 3200 },
];

export const handlers = [
  http.get('*/api/v1/power/status', () => HttpResponse.json(MOCK_STATUS)),
  http.get('*/api/v1/power/history', () => HttpResponse.json(MOCK_HISTORY)),
];
