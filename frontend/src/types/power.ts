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
  device_id: string;
  reading_timestamp: string;
  charge_percentage: number;   // 0–100
  state_of_health: number;
  power_flowing: number;       // watts
  power_direction: 'charging' | 'discharging';
  capacity_wh: number;
}

export interface PowerEvent {
  device_id: string;
  timestamp: string;
  power_produced: number;
  power_consumed: number;
  power_net: number;
  battery_charge?: number;
  battery_w?: number;
  battery_direction?: 'charging' | 'discharging';
}

export type HistoryInterval = 'hour' | 'day' | 'week' | 'month';
