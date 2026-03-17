import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../tests/setup';
import { useBatteryStatus } from '../useBatteryStatus';
import { MOCK_DEVICE_ID, MOCK_BATTERY } from '../../tests/handlers';
import { PowerEvent } from '../../types/power';

describe('useBatteryStatus', () => {
  it('fetches battery status on mount', async () => {
    const { result } = renderHook(() => useBatteryStatus(MOCK_DEVICE_ID));
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.battery?.charge_percentage).toBe(75.0);
    expect(result.current.battery?.power_direction).toBe('charging');
    expect(result.current.error).toBeNull();
  });

  it('returns null battery when server returns no data', async () => {
    server.use(
      http.get('*/api/v1/power/battery', () => HttpResponse.json({ status: 'no data' }))
    );

    const { result } = renderHook(() => useBatteryStatus(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.battery).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('updates power_flowing and direction from SSE event', async () => {
    const { result, rerender } = renderHook(
      ({ event }: { event?: PowerEvent | null }) => useBatteryStatus(MOCK_DEVICE_ID, event),
      { initialProps: { event: undefined } }
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.battery?.power_flowing).toBe(800);

    const liveEvent: PowerEvent = {
      device_id: MOCK_DEVICE_ID,
      timestamp: '2024-06-01T12:01:00Z',
      power_produced: 4000,
      power_consumed: 2000,
      power_net: 2000,
      battery_w: 1200,
      battery_direction: 'charging',
    };

    rerender({ event: liveEvent });

    expect(result.current.battery?.power_flowing).toBe(1200);
    expect(result.current.battery?.power_direction).toBe('charging');
    // charge_percentage unchanged by SSE — only power_flowing updates
    expect(result.current.battery?.charge_percentage).toBe(75.0);
  });

  it('ignores SSE events without battery_w', async () => {
    const { result, rerender } = renderHook(
      ({ event }: { event?: PowerEvent | null }) => useBatteryStatus(MOCK_DEVICE_ID, event),
      { initialProps: { event: undefined } }
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    const initialFlowing = result.current.battery?.power_flowing;

    // Event with no battery_w should not trigger an update
    const eventWithoutBattery: PowerEvent = {
      device_id: MOCK_DEVICE_ID,
      timestamp: '2024-06-01T12:01:00Z',
      power_produced: 4000,
      power_consumed: 2000,
      power_net: 2000,
    };

    rerender({ event: eventWithoutBattery });
    expect(result.current.battery?.power_flowing).toBe(initialFlowing);
  });

  it('surfaces fetch error in error state', async () => {
    server.use(http.get('*/api/v1/power/battery', () => HttpResponse.error()));

    const { result } = renderHook(() => useBatteryStatus(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeTruthy();
    expect(result.current.battery).toBeNull();
  });
});
