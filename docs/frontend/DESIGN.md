# Frontend Design Document — Power Monitor Dashboard

## Concept: "Deep Current"

The aesthetic metaphor is **electricity as a living system** — instruments that show power flowing through a home in real time, not just numbers on a screen. Inspired by Braun industrial design and submarine navigation panels: dark, precise, purposeful. No eco-clichés (no green leaves, no cartoon suns). The interface treats energy data the way a physicist would — as measurable current with direction and magnitude.

The single element that makes this unforgettable: **an animated energy flow diagram** where dashed SVG lines race from solar panels through an inverter to home, battery, and grid. Line weight changes with wattage in real time. Arrow direction flips when battery switches between charging and discharging.

---

## Aesthetic Decisions

### Theme
**Dark only.** Energy/power data reads better at high contrast against a dark background. Matches the instrument-panel metaphor. Avoids the generic "eco dashboard" look.

### Color Palette

```css
--bg:           #080810   /* near-black with slight blue tint */
--surface:      #0f0f1a   /* card surfaces */
--border:       #1c1c2e   /* subtle separators */
--text-primary: #dcd8f0   /* warm off-white */
--text-dim:     #52506b   /* secondary labels */

--amber:        #f0a500   /* solar production — warm, energetic */
--coral:        #ff5e45   /* home consumption — active, drawing */
--cyan:         #00c8e0   /* grid export — power going out */
--red:          #ff3b30   /* grid import — drawing from grid */
--green:        #30d158   /* battery charging */
--yellow:       #ffd60a   /* battery discharging */
```

**Rationale:** Each power flow gets a distinct, meaningful colour. Amber = sun/warmth. Cyan = export (cool, outward). Red = import (alert/cost). Green/yellow = battery state. No colour appears twice for different meanings.

### Typography

| Use | Font | Weight | Rationale |
|-----|------|--------|-----------|
| Large metrics (watts) | `Share Tech Mono` | 400 | Monospace keeps digits stable as values update; technical precision |
| Labels, UI text | `Barlow Condensed` | 400/600 | Industrial, compact, legible at small sizes |
| Section headings | `Barlow Condensed` | 700, letter-spaced | Matches instrument-panel aesthetic |

Both fonts are Google Fonts — no build dependencies.

**Why not Inter/Roboto:** Too generic. The monospace metric numbers and condensed labels together create a distinctive instrument-panel feel that Inter cannot.

### Motion

| Element | Animation | Purpose |
|---------|-----------|---------|
| Flow diagram lines | CSS `stroke-dashoffset` loop | Show direction and magnitude of current |
| Metric values | CSS counter-style transition (number roll) | Makes value updates feel live, not jarring |
| Live indicator dot | CSS `scale` pulse, 2s loop | Ambient confirmation of SSE connection |
| Battery bar | CSS `width` transition, 600ms ease | Smooth state changes |
| Card values on SSE push | Brief amber `color` flash, 300ms | Draws eye to changed data without distraction |

All animations are CSS-only or React state transitions — no animation library dependency.

### Layout

Single-page, no routing. Three stacked sections:

```
┌──────────────────────────────────────────────────────┐
│ HEADER — branding, device label, live indicator      │
├──────────────────────────────────────────────────────┤
│ METRIC STRIP — 4 cards across full width             │
│  [Solar]  [Consuming]  [Grid ±]  [Battery]           │
├──────────────────────────────────────────────────────┤
│ ENERGY FLOW DIAGRAM — SVG, animated current paths    │
│  Solar → Inverter → Home                             │
│                   → Grid (export/import)             │
│                   → Battery (charge/discharge)       │
├──────────────────────────────────────────────────────┤
│ HISTORY CHART — area chart with interval toggle      │
│  [HOUR] [DAY] [WEEK] [MONTH]                         │
│  Solar (amber) vs Consumption (coral) area series    │
└──────────────────────────────────────────────────────┘
```

**Why no sidebar:** Single device for MVP; navigation is not needed. The vertical flow matches how energy moves — generation at top, history at bottom.

---

## Technical Decisions

### Framework
**React 18 + TypeScript.** Already scaffolded in `frontend/`. Strict TypeScript throughout — no `any`.

### Charting
**Recharts.** Reasons:
- React-native (no imperative DOM manipulation)
- Small bundle (~60 KB gzipped)
- `ResponsiveContainer` handles resize without custom code
- Custom `<Tooltip>` and `<Legend>` are straightforward to style
- Alternative (Chart.js) requires a wrapper and is harder to theme

### State Management
**`useState` + custom hooks only.** No Redux, no Zustand. The data model has two independent concerns:
1. Current status (updated by SSE push)
2. Historical data (fetched on interval-change)

These don't share state, so global state management adds complexity without benefit.

### Real-Time Updates
**Native `EventSource` API** in a `useSSE` hook.
- No library needed
- Browser reconnects automatically on disconnect
- Hook closes the connection on component unmount via cleanup function
- SSE events update the same `currentStatus` state as the REST poll, so there is one source of truth

### Data Fetching
**Native `fetch`** wrapped in typed service functions in `services/api.ts`. No Axios — fetch is sufficient for GET-only endpoints. Errors surfaced via hook state (`{ data, loading, error }`).

### Styling
**CSS Modules + CSS custom properties.** Reasons:
- Scoped class names prevent collisions
- CSS variables allow the design token palette to be defined once
- No runtime CSS-in-JS overhead
- Tailwind would require a build plugin and fights with the custom typography/motion approach

### Energy Flow Diagram
**Hand-authored SVG inside a React component.** Recharts and D3 are overkill for a fixed-topology graph (solar → inverter → 3 destinations). SVG paths are static; only `stroke-dashoffset` animation speed and `strokeWidth` change with data.

---

## API Integration

| Endpoint | Used by | When |
|----------|---------|------|
| `GET /api/v1/power/status?device_id=` | `usePowerStatus` | On mount (initial load) |
| `GET /api/v1/events` (SSE) | `useSSE` | Persistent connection, updates `currentStatus` |
| `GET /api/v1/power/history?device_id=&interval=&start=&end=` | `usePowerHistory` | On mount + when interval/range changes |

`device_id` is hardcoded for MVP (single device). The `DeviceSelector` in the header is a static label until multi-device support is added.

---

## Accessibility

- All metric cards have `aria-label` with full text value (e.g. "Solar production: 5234 watts")
- Color is never the only signal — direction indicators (↑ ↓ ±) accompany all colour-coded values
- Animations respect `prefers-reduced-motion`: flow diagram uses static arrows, pulse dot is hidden
- Minimum contrast ratio 4.5:1 for all text against backgrounds

---

## What Is Explicitly Out of Scope (MVP)

| Feature | Reason deferred |
|---------|----------------|
| Multi-device support | Backend DeviceRepository needs wiring first |
| Authentication / login | Not in backend yet |
| Alerts panel | `alerts` table exists but no API endpoint yet |
| Mobile-optimised layout | Desktop-first for MVP; responsive breakpoints added later |
| Dark/light toggle | Dark-only is intentional for instrument aesthetic |
| Settings page | No user-configurable options in MVP |
