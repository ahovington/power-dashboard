const DEVICE_ID = import.meta.env.VITE_DEVICE_ID ?? 'replace-with-device-id';

export default function App() {
  return <div data-testid="app">{DEVICE_ID}</div>;
}
