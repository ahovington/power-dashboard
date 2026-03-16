import { render, screen, waitFor } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { MOCK_DEVICE_ID } from '../../tests/handlers';

describe('Dashboard', () => {
  it('renders all three metric cards after data loads', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);

    await waitFor(() => {
      expect(screen.getByText('SOLAR')).toBeInTheDocument();
      expect(screen.getByText('CONSUMING')).toBeInTheDocument();
      expect(screen.getByText('GRID')).toBeInTheDocument();
    });
  });

  it('shows correct solar wattage from API', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);
    await waitFor(() => expect(screen.getByText('5,234')).toBeInTheDocument());
  });

  it('renders energy flow diagram', async () => {
    render(<Dashboard deviceId={MOCK_DEVICE_ID} />);
    await waitFor(() =>
      expect(screen.getByRole('img', { name: /energy flow/i })).toBeInTheDocument()
    );
  });
});
