import { useState, useEffect } from 'react';

interface SSEState<T> {
  latestEvent: T | null;
  connected: boolean;
  error: string | null;
}

export function useSSE<T>(url: string): SSEState<T> {
  const [state, setState] = useState<SSEState<T>>({
    latestEvent: null,
    connected: false,
    error: null,
  });

  useEffect(() => {
    const es = new EventSource(url);

    es.onopen = () => setState(s => ({ ...s, connected: true, error: null }));

    es.onmessage = (e: MessageEvent) => {
      try {
        const event = JSON.parse(e.data) as T;
        setState(s => ({ ...s, latestEvent: event }));
      } catch {
        // malformed event — ignore, keep connection alive
      }
    };

    es.onerror = () => {
      setState(s => ({ ...s, connected: false, error: 'SSE connection lost — reconnecting' }));
    };

    return () => {
      es.close();
    };
  }, [url]);

  return state;
}
