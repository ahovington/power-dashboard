import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HistoryChart } from '../HistoryChart';
import { MOCK_HISTORY } from '../../tests/handlers';

describe('HistoryChart', () => {
  it('renders interval toggle buttons', () => {
    render(
      <HistoryChart
        readings={MOCK_HISTORY}
        interval="hour"
        onIntervalChange={vi.fn()}
        loading={false}
      />
    );
    expect(screen.getByRole('button', { name: 'HOUR' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'DAY' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'WEEK' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'MONTH' })).toBeInTheDocument();
  });

  it('calls onIntervalChange when a button is clicked', async () => {
    const onChange = vi.fn();
    render(
      <HistoryChart
        readings={MOCK_HISTORY}
        interval="hour"
        onIntervalChange={onChange}
        loading={false}
      />
    );
    await userEvent.click(screen.getByRole('button', { name: 'DAY' }));
    expect(onChange).toHaveBeenCalledWith('day');
  });

  it('renders loading skeleton when loading=true', () => {
    render(
      <HistoryChart
        readings={[]}
        interval="hour"
        onIntervalChange={vi.fn()}
        loading={true}
      />
    );
    expect(screen.getByTestId('chart-skeleton')).toBeInTheDocument();
  });
});
