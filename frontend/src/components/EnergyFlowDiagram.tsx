import styles from './EnergyFlowDiagram.module.css';

interface EnergyFlowDiagramProps {
  solarW: number;
  consumedW: number;
  netW: number;
  batteryW: number;
  batteryDirection: 'charging' | 'discharging';
}

// Maps wattage to stroke width (1–6px range)
function strokeWeight(watts: number): number {
  const clamped = Math.min(Math.abs(watts), 10000);
  return 1 + (clamped / 10000) * 5;
}

// Maps wattage to animation duration (slower = less power, faster = more power)
function animDuration(watts: number): string {
  const clamped = Math.max(Math.abs(watts), 100);
  const secs = 2 - (clamped / 10000) * 1.5;
  return `${Math.max(secs, 0.5).toFixed(2)}s`;
}

function fmt(w: number): string {
  return `${(Math.abs(w) / 1000).toFixed(1)} kW`;
}

export function EnergyFlowDiagram({
  solarW,
  consumedW,
  netW,
  batteryW,
  batteryDirection,
}: EnergyFlowDiagramProps) {
  const isExporting = netW >= 0;
  const gridColor = isExporting ? 'var(--cyan)' : 'var(--red)';
  const batteryColor = batteryDirection === 'charging' ? 'var(--green)' : 'var(--yellow)';
  const dashArray = '12 8';

  // Layout: Solar (left) → Inverter (centre) → Home/Grid/Battery (right)
  //   Solar:    x=60,  y=200
  //   Inverter: x=220, y=200
  //   Home:     x=380, y=120
  //   Grid:     x=380, y=200
  //   Battery:  x=380, y=280

  return (
    <div className={styles.container}>
      <svg
        width="100%"
        viewBox="0 0 460 360"
        role="img"
        aria-label="Energy flow diagram"
        style={{ maxWidth: 460 }}
      >
        {/* ── Paths ── */}
        {/* Solar → Inverter */}
        <path
          d="M 100 200 L 200 200"
          className={`${styles.flowLine} ${styles.flowAnimated}`}
          stroke="var(--amber)"
          strokeWidth={strokeWeight(solarW)}
          strokeDasharray={dashArray}
          style={{ animationDuration: animDuration(solarW) }}
        />

        {/* Inverter → Home */}
        <path
          d="M 240 200 L 310 200 L 310 130 L 340 130"
          className={`${styles.flowLine} ${styles.flowAnimated}`}
          stroke="var(--coral)"
          strokeWidth={strokeWeight(consumedW)}
          strokeDasharray={dashArray}
          style={{ animationDuration: animDuration(consumedW) }}
        />

        {/* Inverter → Grid (direction flips on import) */}
        <path
          d="M 240 200 L 340 200"
          className={`${styles.flowLine} ${isExporting ? styles.flowAnimated : styles.flowReverse}`}
          stroke={gridColor}
          strokeWidth={strokeWeight(netW)}
          strokeDasharray={dashArray}
          style={{ animationDuration: animDuration(netW) }}
        />

        {/* Inverter → Battery (direction flips on discharge) */}
        <path
          d="M 240 200 L 310 200 L 310 270 L 340 270"
          className={`${styles.flowLine} ${batteryDirection === 'charging' ? styles.flowAnimated : styles.flowReverse}`}
          stroke={batteryColor}
          strokeWidth={strokeWeight(batteryW)}
          strokeDasharray={dashArray}
          style={{ animationDuration: animDuration(batteryW) }}
        />

        {/* ── Nodes ── */}

        {/* Solar */}
        <g className={styles.node}>
          <rect x="10" y="172" width="90" height="56" rx="4" fill="var(--surface)" stroke="var(--border)" />
          <text x="55" y="192" className={styles.nodeLabel} fill="var(--amber)">Solar</text>
          <text x="55" y="212" className={styles.nodeValue}>{fmt(solarW)}</text>
        </g>

        {/* Inverter */}
        <g className={styles.node}>
          <rect x="200" y="180" width="40" height="40" rx="4" fill="var(--border)" stroke="var(--border)" />
          <text x="220" y="205" className={styles.nodeLabel} fill="var(--text-dim)" fontSize="9">INV</text>
        </g>

        {/* Home */}
        <g className={styles.node}>
          <rect x="340" y="102" width="100" height="56" rx="4" fill="var(--surface)" stroke="var(--border)" />
          <text x="390" y="122" className={styles.nodeLabel} fill="var(--coral)">Home</text>
          <text x="390" y="142" className={styles.nodeValue}>{fmt(consumedW)}</text>
        </g>

        {/* Grid */}
        <g className={styles.node}>
          <rect x="340" y="172" width="100" height="56" rx="4" fill="var(--surface)" stroke={gridColor} />
          <text x="390" y="192" className={styles.nodeLabel} fill={gridColor}>Grid</text>
          <text x="390" y="208" className={`${styles.gridLabel}`} fill={gridColor}>
            {isExporting ? 'EXPORTING' : 'IMPORTING'}
          </text>
        </g>

        {/* Battery */}
        <g className={styles.node}>
          <rect x="340" y="242" width="100" height="56" rx="4" fill="var(--surface)" stroke={batteryColor} />
          <text x="390" y="262" className={styles.nodeLabel} fill={batteryColor}>Battery</text>
          <text x="390" y="282" className={styles.nodeValue}>{fmt(batteryW)}</text>
        </g>
      </svg>
    </div>
  );
}
