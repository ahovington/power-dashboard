import { renderHook, waitFor, act } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../tests/setup';
import { usePowerHistory } from '../usePowerHistory';
import { MOCK_DEVICE_ID } from '../../tests/handlers';

describe('usePowerHistory', () => {
  it('fetches history on mount with default interval', async () => {
    const { result } = renderHook(() => usePowerHistory(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.readings).toHaveLength(3);
  });

  it('re-fetches when interval changes', async () => {
    const { result } = renderHook(() => usePowerHistory(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.setHistoryInterval('day'));
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.readings).toBeDefined();
  });

  it('surfaces fetch error in error state', async () => {
    server.use(http.get('*/api/v1/power/history', () => HttpResponse.error()));
    const { result } = renderHook(() => usePowerHistory(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeTruthy();
    expect(result.current.readings).toHaveLength(0);
  });

  it('sends correct date range for each interval', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    const { result } = renderHook(() => usePowerHistory(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.setHistoryInterval('day'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Find the last call that hit /power/history
    const historyCall = [...fetchSpy.mock.calls]
      .reverse()
      .find(([url]) => String(url).includes('power/history'));
    expect(historyCall).toBeDefined();

    const url = new URL(String(historyCall![0]));
    const start = new Date(url.searchParams.get('start')!);
    const end = new Date(url.searchParams.get('end')!);
    const diffHours = (end.getTime() - start.getTime()) / (1000 * 60 * 60);

    expect(diffHours).toBeGreaterThan(6 * 24); // at least 6 days
    expect(diffHours).toBeLessThan(8 * 24);    // at most 8 days
  });
});
