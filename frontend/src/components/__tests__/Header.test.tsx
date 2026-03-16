import { render, screen } from '@testing-library/react';
import { Header } from '../Header';

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
