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
    render(
      <MetricCard
        label="SOLAR"
        value={5234}
        unit="W"
        accent="amber"
        subtitle="peak today: 6.1 kW"
      />
    );
    expect(screen.getByText('peak today: 6.1 kW')).toBeInTheDocument();
  });

  it('renders direction indicator when provided', () => {
    render(
      <MetricCard label="GRID" value={2134} unit="W" accent="cyan" direction="export" />
    );
    expect(screen.getByRole('img', { name: /exporting/i })).toBeInTheDocument();
  });
});
