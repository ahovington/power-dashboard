import { render, screen } from '@testing-library/react';
import { EnergyFlowDiagram } from '../EnergyFlowDiagram';

describe('EnergyFlowDiagram', () => {
  it('renders solar, home, grid, and battery nodes', () => {
    render(
      <EnergyFlowDiagram
        solarW={5234}
        consumedW={3100}
        netW={2134}
        batteryW={420}
        batteryDirection="charging"
      />
    );
    expect(screen.getByText(/solar/i)).toBeInTheDocument();
    expect(screen.getByText(/home/i)).toBeInTheDocument();
    expect(screen.getByText(/grid/i)).toBeInTheDocument();
    expect(screen.getByText(/battery/i)).toBeInTheDocument();
  });

  it('shows EXPORTING label when netW is positive', () => {
    render(
      <EnergyFlowDiagram
        solarW={5000}
        consumedW={3000}
        netW={2000}
        batteryW={0}
        batteryDirection="charging"
      />
    );
    expect(screen.getByText(/exporting/i)).toBeInTheDocument();
  });

  it('shows IMPORTING label when netW is negative', () => {
    render(
      <EnergyFlowDiagram
        solarW={1000}
        consumedW={3000}
        netW={-2000}
        batteryW={0}
        batteryDirection="discharging"
      />
    );
    expect(screen.getByText(/importing/i)).toBeInTheDocument();
  });
});
