import { renderHook, act } from '@testing-library/react';
import { useSSE } from '../useSSE';
import { PowerEvent } from '../../types/power';

// Mock EventSource — jsdom does not implement it
class MockEventSource {
  static instances: MockEventSource[] = [];
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onopen: (() => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {
    MockEventSource.instances.push(this);
  }
  emit(data: string) {
    this.onmessage?.({ data } as MessageEvent);
  }
}
(globalThis as unknown as { EventSource: unknown }).EventSource = MockEventSource;

describe('useSSE', () => {
  beforeEach(() => {
    MockEventSource.instances = [];
  });

  it('updates latestEvent when a message arrives', () => {
    const { result } = renderHook(() => useSSE<PowerEvent>('http://localhost/events'));
    const es = MockEventSource.instances[0];

    const event: PowerEvent = {
      device_id: 'abc',
      timestamp: '2024-06-01T12:00:00.000Z',
      power_produced: 5000,
      power_consumed: 3000,
      power_net: 2000,
    };

    act(() => {
      es.emit(JSON.stringify(event));
    });

    expect(result.current.latestEvent?.power_produced).toBe(5000);
  });

  it('closes EventSource on unmount', () => {
    const { unmount } = renderHook(() => useSSE<PowerEvent>('http://localhost/events'));
    const es = MockEventSource.instances[0];
    unmount();
    expect(es.close).toHaveBeenCalledTimes(1);
  });

  it('sets error state when EventSource fires onerror', () => {
    const { result } = renderHook(() => useSSE<PowerEvent>('http://localhost/events'));
    const es = MockEventSource.instances[0];
    act(() => {
      es.onerror?.(new Event('error'));
    });
    expect(result.current.error).toBeTruthy();
  });
});
