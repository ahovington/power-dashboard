import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../tests/setup';
import { usePowerStatus } from '../usePowerStatus';
import { MOCK_DEVICE_ID, MOCK_STATUS } from '../../tests/handlers';
import { PowerEvent } from '../../types/power';

describe('usePowerStatus', () => {
  it('fetches status on mount', async () => {
    const { result } = renderHook(() => usePowerStatus(MOCK_DEVICE_ID));
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.status?.power_produced).toBe(5234);
    expect(result.current.error).toBeNull();
  });

  it('merges SSE event into status when liveEvent prop changes', async () => {
    const liveEvent: PowerEvent = {
      device_id: MOCK_DEVICE_ID,
      timestamp: '2024-06-01T12:01:00.000Z',
      power_produced: 9999,
      power_consumed: 4000,
      power_net: 5999,
    };

    const { result, rerender } = renderHook(
      ({ event }: { event?: PowerEvent | null }) => usePowerStatus(MOCK_DEVICE_ID, event),
      { initialProps: { event: undefined } }
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.status?.power_produced).toBe(5234);

    rerender({ event: liveEvent });

    expect(result.current.status?.power_produced).toBe(9999);
  });

  it('surfaces fetch error in error state', async () => {
    server.use(http.get('*/api/v1/power/status', () => HttpResponse.error()));

    const { result } = renderHook(() => usePowerStatus(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeTruthy();
    expect(result.current.status).toBeNull();
  });
});
