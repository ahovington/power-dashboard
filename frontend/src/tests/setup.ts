import '@testing-library/jest-dom';
import { setupServer } from 'msw/node';
import { handlers } from './handlers';

// Recharts uses ResizeObserver — polyfill for jsdom
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// jsdom does not implement EventSource — provide a no-op stub so
// components that call useSSE don't throw in integration tests.
// Individual useSSE tests override this with a full mock.
class StubEventSource {
  static instances: StubEventSource[] = [];
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onopen: (() => void) | null = null;
  close = vi.fn();
  constructor(_url: string) { StubEventSource.instances.push(this); }
}
(globalThis as unknown as { EventSource: unknown }).EventSource = StubEventSource;

export const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
