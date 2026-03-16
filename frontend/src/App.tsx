import { Dashboard } from './components/Dashboard';

const DEVICE_ID = import.meta.env.VITE_DEVICE_ID ?? 'replace-with-device-id';

export default function App() {
  return <Dashboard deviceId={DEVICE_ID} />;
}
