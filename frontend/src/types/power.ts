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

export interface PowerEvent {
  device_id: string;
  timestamp: string;
  power_produced: number;
  power_consumed: number;
  power_net: number;
  battery_charge?: number;
}

export type HistoryInterval = 'hour' | 'day' | 'week' | 'month';
