# Frontend Implementation Plan (TDD)

Step-by-step guide for building the Power Monitor dashboard frontend using Test-Driven Development.

**TDD cycle:** Red → Green → Refactor. Write the test first, watch it fail, write the minimum code to pass, clean up.

Run `npm test -- --watchAll=false` after every green step.

---

## Step 1 — Project Scaffolding & Dependencies

No tests yet — pure setup.

### `frontend/package.json`

> **Toolchain: Vite** (not Create React App — CRA is unmaintained since 2023)

```json
{
  "name": "power-monitor-frontend",
  "version": "0.1.0",
  "private": true,
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "recharts": "^2.10.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.1.0",
    "@testing-library/react": "^14.1.0",
    "@testing-library/user-event": "^14.5.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "jsdom": "^24.0.0",
    "msw": "^2.0.0",
    "typescript": "^5.3.0",
    "vite": "^5.1.0",
    "vitest": "^1.3.0"
  },
  "scripts": {
    "start": "vite",
    "build": "vite build",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:coverage": "vitest run --coverage"
  }
}
```

### `frontend/vite.config.ts`

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/tests/setup.ts',
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
```

```bash
cd frontend && npm install
```

### Styling architecture

**CSS Modules + CSS custom properties.** Each component gets a `ComponentName.module.css` file alongside it. Classes are scoped automatically by Vite. Design tokens from `tokens.css` (custom properties) are available globally to all module files.

```
Component.tsx          ← imports styles from Component.module.css
Component.module.css   ← uses var(--amber) etc. from tokens.css
```

### `frontend/src/styles/tokens.css`

Global CSS design tokens — imported once in `index.tsx`.

```css
@import url('https://fonts.googleapis.com/css2?family=Share+Tech+Mono&family=Barlow+Condensed:wght@400;600;700&display=swap');

:root {
  --bg:           #080810;
  --surface:      #0f0f1a;
  --border:       #1c1c2e;
  --text-primary: #dcd8f0;
  --text-dim:     #52506b;

  --amber:  #f0a500;
  --coral:  #ff5e45;
  --cyan:   #00c8e0;
  --red:    #ff3b30;
  --green:  #30d158;
  --yellow: #ffd60a;

  --font-mono:      'Share Tech Mono', monospace;
  --font-condensed: 'Barlow Condensed', sans-serif;
}

*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

body {
  background: var(--bg);
  color: var(--text-primary);
  font-family: var(--font-condensed);
  -webkit-font-smoothing: antialiased;
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation: none !important; transition: none !important; }
}
```

---

## Step 2 — Types & API Service Layer

No tests yet — foundation all other tests depend on.

### `frontend/src/types/power.ts`

```typescript
export interface PowerStatus {
  device_id: string;
  timestamp: string;
  power_produced: number;   // watts, solar
  power_consumed: number;   // watts, home
  power_net: number;        // watts, positive = exporting, negative = importing
}

export interface PowerReading {
  reading_timestamp: string;
  power_produced: number;
  power_consumed: number;
}

export interface BatteryStatus {
  charge_percentage: number;     // 0–100
  power_flowing: number;         // watts
  power_direction: 'charging' | 'discharging';
}

export interface PowerEvent {
  device_id: string;
  timestamp: string;
  power_produced: number;
  power_consumed: number;
  power_net: number;
  battery_charge?: number;
}

export type HistoryInterval = 'hour' | 'day' | 'week' | 'month';
```

### `frontend/src/services/api.ts`

```typescript
import { PowerStatus, PowerReading, HistoryInterval } from '../types/power';

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

export const SSE_URL = `${BASE_URL}/api/v1/events`;
```

---

## Step 3 — MSW Handlers (test mock server)

### `frontend/src/tests/handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { PowerStatus, PowerReading } from '../types/power';

export const MOCK_DEVICE_ID = 'test-device-uuid';

export const MOCK_STATUS: PowerStatus = {
  device_id: MOCK_DEVICE_ID,
  timestamp: '2024-06-01T12:00:00.000Z', // fixed — non-deterministic new Date() causes flaky timestamp assertions
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
```

### `frontend/src/tests/setup.ts`

```typescript
import { setupServer } from 'msw/node';
import { handlers } from './handlers';

export const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

---

## Step 4 — `useSSE` Hook (TDD)

### Red: `frontend/src/hooks/__tests__/useSSE.test.ts`

```typescript
import { renderHook, act } from '@testing-library/react';
import { useSSE } from '../useSSE';
import { PowerEvent } from '../../types/power';

// Mock EventSource
class MockEventSource {
  static instances: MockEventSource[] = [];
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  close = jest.fn();
  constructor(public url: string) { MockEventSource.instances.push(this); }
  emit(data: string) { this.onmessage?.({ data } as MessageEvent); }
}
(global as any).EventSource = MockEventSource;

describe('useSSE', () => {
  beforeEach(() => { MockEventSource.instances = []; });

  it('updates latestEvent when a message arrives', () => {
    const { result } = renderHook(() => useSSE<PowerEvent>('http://localhost/events'));
    const es = MockEventSource.instances[0];

    const event: PowerEvent = {
      device_id: 'abc', timestamp: new Date().toISOString(),
      power_produced: 5000, power_consumed: 3000, power_net: 2000,
    };

    act(() => { es.emit(JSON.stringify(event)); });

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
    act(() => { es.onerror?.(new Event('error')); });
    expect(result.current.error).toBeTruthy();
  });
});
```

### Green: `frontend/src/hooks/useSSE.ts`

```typescript
import { useState, useEffect, useRef } from 'react';

interface SSEState<T> {
  latestEvent: T | null;
  connected: boolean;
  error: string | null;
}

export function useSSE<T>(url: string): SSEState<T> {
  const [state, setState] = useState<SSEState<T>>({
    latestEvent: null, connected: false, error: null,
  });
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const es = new EventSource(url);
    esRef.current = es;

    es.onopen = () => setState(s => ({ ...s, connected: true, error: null }));

    es.onmessage = (e: MessageEvent) => {
      try {
        const event = JSON.parse(e.data) as T;
        setState(s => ({ ...s, latestEvent: event }));
      } catch {
        // malformed event — ignore, keep connection
      }
    };

    es.onerror = () => {
      setState(s => ({ ...s, connected: false, error: 'SSE connection lost — reconnecting' }));
    };

    return () => { es.close(); };
  }, [url]);

  return state;
}
```

---

## Step 5 — `usePowerStatus` Hook (TDD)

### Red: `frontend/src/hooks/__tests__/usePowerStatus.test.ts`

```typescript
import { renderHook, waitFor } from '@testing-library/react';
import '../../../src/tests/setup';
import { usePowerStatus } from '../usePowerStatus';
import { MOCK_DEVICE_ID, MOCK_STATUS } from '../../tests/handlers';

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
    expect(result.current.status?.power_produced).toBe(5234); // initial fetch value

    rerender({ event: liveEvent });

    expect(result.current.status?.power_produced).toBe(9999); // merged from SSE
  });

  it('surfaces fetch error in error state', async () => {
    // override handler to return 500
    server.use(http.get('*/api/v1/power/status', () => HttpResponse.error()));
    const { result } = renderHook(() => usePowerStatus(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeTruthy();
    expect(result.current.status).toBeNull();
  });
});
```

### Green: `frontend/src/hooks/usePowerStatus.ts`

```typescript
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
    status: null, loading: true, error: null,
  });

  // Initial fetch
  useEffect(() => {
    fetchPowerStatus(deviceId)
      .then(status => setState({ status, loading: false, error: null }))
      .catch(err => setState({ status: null, loading: false, error: String(err) }));
  }, [deviceId]);

  // Merge live SSE event into status without re-fetching
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
```

---

## Step 6 — `usePowerHistory` Hook (TDD)

### Red: `frontend/src/hooks/__tests__/usePowerHistory.test.ts`

```typescript
import { renderHook, waitFor, act } from '@testing-library/react';
import { usePowerHistory } from '../usePowerHistory';
import { MOCK_DEVICE_ID, MOCK_HISTORY } from '../../tests/handlers';

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
    const fetchSpy = vi.spyOn(global, 'fetch');
    const { result } = renderHook(() => usePowerHistory(MOCK_DEVICE_ID));
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Switch to 'day' — should request ~7 days back, not 7 hours
    act(() => result.current.setHistoryInterval('day'));
    await waitFor(() => expect(result.current.loading).toBe(false));

    const url = new URL(fetchSpy.mock.calls.at(-1)![0] as string);
    const start = new Date(url.searchParams.get('start')!);
    const end = new Date(url.searchParams.get('end')!);
    const diffHours = (end.getTime() - start.getTime()) / (1000 * 60 * 60);
    expect(diffHours).toBeGreaterThan(6 * 24); // at least 6 days
    expect(diffHours).toBeLessThan(8 * 24);    // at most 8 days
  });
});
```

### Green: `frontend/src/hooks/usePowerHistory.ts`

```typescript
import { useState, useEffect, useCallback } from 'react';
import { PowerReading, HistoryInterval } from '../types/power';
import { fetchPowerHistory } from '../services/api';

interface PowerHistoryState {
  readings: PowerReading[];
  loading: boolean;
  error: string | null;
  interval: HistoryInterval;
  setHistoryInterval: (i: HistoryInterval) => void; // renamed: avoids shadowing window.setInterval
}

// Returns the correct start date for each interval type.
// Uses setDate/setMonth for day/week/month — NOT setHours — to avoid the
// "7 hours instead of 7 days" bug that setHours would produce.
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
    readings: [], loading: true, error: null,
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
```

---

## Step 7 — `MetricCard` Component (TDD)

### Red: `frontend/src/components/__tests__/MetricCard.test.tsx`

```typescript
import { render, screen } from '@testing-library/react';
import { MetricCard } from '../MetricCard';

describe('MetricCard', () => {
  it('renders label and formatted value', () => {
    render(<MetricCard label="SOLAR" value={5234} unit="W" accent="amber" />);
    expect(screen.getByText('SOLAR')).toBeInTheDocument();
    expect(screen.getByText('5,234')).toBeInTheDocument();
    expect(screen.getByText('W')).toBeInTheDocument();
  });

  it('renders subtitle when provided', () => {
    render(<MetricCard label="SOLAR" value={5234} unit="W" accent="amber" subtitle="peak today: 6.1 kW" />);
    expect(screen.getByText('peak today: 6.1 kW')).toBeInTheDocument();
  });

  it('renders direction indicator when provided', () => {
    render(<MetricCard label="GRID" value={2134} unit="W" accent="cyan" direction="export" />);
    expect(screen.getByRole('img', { name: /exporting/i })).toBeInTheDocument();
  });
});
```

### Green: `frontend/src/components/MetricCard.tsx`

Props: `label`, `value` (number, watts), `unit`, `accent` (colour token key), `subtitle?`, `direction?` ('export' | 'import' | 'charge' | 'discharge').

Renders:
- Label in `Barlow Condensed` 700, letter-spaced, `--text-dim`
- Value in `Share Tech Mono`, large, coloured by `accent`
- Unit in small `Barlow Condensed`
- Optional subtitle in `--text-dim`
- Optional direction arrow (`↑` export/charge, `↓` import/discharge)
- Brief amber flash animation on value change (via `useEffect` + class toggle)

> **BatteryCard deferred from MVP.** The backend `GetBatteryStatus()` returns `nil, nil` for Enphase — no battery data is available. Removed to avoid permanently-null UI. See TODOS.md: "Wire BatteryCard once backend exposes battery data."

---

## Step 8 — `EnergyFlowDiagram` Component (TDD)

### Red: `frontend/src/components/__tests__/EnergyFlowDiagram.test.tsx`

```typescript
describe('EnergyFlowDiagram', () => {
  it('renders solar, home, grid, and battery nodes', () => {
    render(<EnergyFlowDiagram
      solarW={5234} consumedW={3100} netW={2134} batteryW={420} batteryDirection="charging"
    />);
    expect(screen.getByText(/solar/i)).toBeInTheDocument();
    expect(screen.getByText(/home/i)).toBeInTheDocument();
    expect(screen.getByText(/grid/i)).toBeInTheDocument();
    expect(screen.getByText(/battery/i)).toBeInTheDocument();
  });

  it('shows EXPORTING label when netW is positive', () => {
    render(<EnergyFlowDiagram solarW={5000} consumedW={3000} netW={2000} batteryW={0} batteryDirection="charging" />);
    expect(screen.getByText(/exporting/i)).toBeInTheDocument();
  });

  it('shows IMPORTING label when netW is negative', () => {
    render(<EnergyFlowDiagram solarW={1000} consumedW={3000} netW={-2000} batteryW={0} batteryDirection="discharging" />);
    expect(screen.getByText(/importing/i)).toBeInTheDocument();
  });
});
```

### Green: `frontend/src/components/EnergyFlowDiagram.tsx`

- Fixed SVG viewBox, responsive via `width="100%"`
- Nodes: Solar (left), Inverter (centre), Home / Grid / Battery (right column)
- Paths: SVG `<path>` elements, `stroke-dasharray` + CSS animation `stroke-dashoffset`
- `strokeWidth` scales linearly with wattage (min 1px, max 6px)
- Flow animation speed scales with wattage (faster = more power)
- Battery path arrow direction reverses based on `batteryDirection`
- Grid path colour: `--cyan` when exporting, `--red` when importing
- `aria-label` on the SVG container

---

## Step 9 — `HistoryChart` Component (TDD)

### Red: `frontend/src/components/__tests__/HistoryChart.test.tsx`

```typescript
describe('HistoryChart', () => {
  it('renders interval toggle buttons', () => {
    render(<HistoryChart readings={MOCK_HISTORY} interval="hour" onIntervalChange={jest.fn()} loading={false} />);
    expect(screen.getByRole('button', { name: 'HOUR' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'DAY' })).toBeInTheDocument();
  });

  it('calls onIntervalChange when a button is clicked', async () => {
    const onChange = jest.fn();
    render(<HistoryChart readings={MOCK_HISTORY} interval="hour" onIntervalChange={onChange} loading={false} />);
    await userEvent.click(screen.getByRole('button', { name: 'DAY' }));
    expect(onChange).toHaveBeenCalledWith('day');
  });

  it('renders loading skeleton when loading=true', () => {
    render(<HistoryChart readings={[]} interval="hour" onIntervalChange={jest.fn()} loading={true} />);
    expect(screen.getByTestId('chart-skeleton')).toBeInTheDocument();
  });
});
```

### Green: `frontend/src/components/HistoryChart.tsx`

- `IntervalToggle` sub-component: four buttons styled as pill tabs, active state highlighted
- `ResponsiveContainer` wrapping `AreaChart` from Recharts
- Two `Area` series: solar (amber, 30% opacity fill) and consumption (coral, 30% opacity fill)
- Custom `Tooltip` showing both values + computed net
- No Recharts default grid lines or legend chrome — clean instrument look
- Loading state: shimmer skeleton div instead of chart

---

## Step 10 — `Header` Component (TDD)

### Red: `frontend/src/components/__tests__/Header.test.tsx`

```typescript
describe('Header', () => {
  it('renders app name', () => {
    render(<Header connected={true} lastUpdated={new Date()} />);
    expect(screen.getByText(/power/i)).toBeInTheDocument();
  });

  it('shows LIVE indicator when connected', () => {
    render(<Header connected={true} lastUpdated={new Date()} />);
    expect(screen.getByText('LIVE')).toBeInTheDocument();
  });

  it('shows RECONNECTING when not connected', () => {
    render(<Header connected={false} lastUpdated={null} />);
    expect(screen.getByText(/reconnecting/i)).toBeInTheDocument();
  });
});
```

### Green: `frontend/src/components/Header.tsx`

- App name in `Barlow Condensed` 700, letter-spaced: `POWER / MONITOR`
- Device label (static for MVP): `HOME SYSTEM`
- Live indicator: pulsing dot + `LIVE` in `--green` when connected; `RECONNECTING` in `--text-dim` when not
- Timestamp of last SSE event in `Share Tech Mono`
- Single horizontal bar, dark border bottom

---

## Step 11 — `Dashboard` Component + `App` Integration (TDD)

### Red: `frontend/src/components/__tests__/Dashboard.test.tsx`

```typescript
describe('Dashboard', () => {
  it('renders all three metric cards after data loads', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);

    await waitFor(() => {
      expect(screen.getByText('SOLAR')).toBeInTheDocument();
      expect(screen.getByText('CONSUMING')).toBeInTheDocument();
      expect(screen.getByText('GRID')).toBeInTheDocument();
      // BATTERY card deferred — no backend data source yet
    });
  });

  it('shows correct solar wattage from API', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);
    await waitFor(() => expect(screen.getByText('5,234')).toBeInTheDocument());
  });

  it('renders energy flow diagram', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);
    await waitFor(() => expect(screen.getByRole('img', { name: /energy flow/i })).toBeInTheDocument());
  });
});
```

### Green: `frontend/src/components/Dashboard.tsx`

Wires all hooks and components together:
- `useSSE(SSE_URL)` → `liveEvent`
- `usePowerStatus(deviceId, liveEvent)` → `status`
- `usePowerHistory(deviceId)` → `{ readings, interval, setHistoryInterval, loading }`
- Renders: `Header` → `MetricStrip` → `EnergyFlowDiagram` → `HistoryChart`

### `frontend/src/App.tsx`

```typescript
import Dashboard from './components/Dashboard';
import './styles/tokens.css';

const DEVICE_ID = import.meta.env.VITE_DEVICE_ID ?? 'replace-with-device-id';

export default function App() {
  return <Dashboard deviceId={DEVICE_ID} />;
}
```

---

## File Structure

```
frontend/src/
├── styles/
│   └── tokens.css               ← design tokens, reset, fonts
├── types/
│   └── power.ts                 ← TypeScript interfaces
├── services/
│   └── api.ts                   ← fetch wrappers
├── hooks/
│   ├── useSSE.ts
│   ├── usePowerStatus.ts
│   ├── usePowerHistory.ts
│   └── __tests__/
│       ├── useSSE.test.ts
│       ├── usePowerStatus.test.ts
│       └── usePowerHistory.test.ts
├── components/
│   ├── Header.tsx
│   ├── MetricCard.tsx
│   ├── EnergyFlowDiagram.tsx
│   ├── HistoryChart.tsx
│   ├── Dashboard.tsx
│   └── __tests__/
│       ├── Header.test.tsx
│       ├── MetricCard.test.tsx
│       ├── EnergyFlowDiagram.test.tsx
│       ├── HistoryChart.test.tsx
│       └── Dashboard.test.tsx
├── tests/
│   ├── handlers.ts              ← MSW mock handlers
│   └── setup.ts                 ← MSW server setup
├── App.tsx
└── index.tsx
```

---

## Test Coverage Targets

| Layer | Target |
|-------|--------|
| Hooks | 85%+ |
| Components | 75%+ |
| Services | 70%+ (via hook tests) |
| Overall | 75%+ |

---

## Build Order (Critical Path)

```
Step 1: Scaffold + tokens (Vite, CSS Modules confirmed)
    ↓
Step 2: Types + API service (foundation)
    ↓
Step 3: MSW handlers (unblocks all hook/component tests)
    ↓
Steps 4–6: Hooks (data layer — unblocks all component tests)
    ↓
Steps 7–10: Leaf components (can be built in parallel after hooks)
    ↓
Step 11: Dashboard + App assembly
```
