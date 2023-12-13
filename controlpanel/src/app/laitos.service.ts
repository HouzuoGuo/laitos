import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { Server } from './config.service';

export interface ProgramStatusSummary {
  PublicIP?: string;
  HostName?: string;
  ClockTime?: string;
  SysUptime?: number;
  ProgramUptime?: number;
  SysTotalMemMB?: number;
  SysUsedMemMB?: number;
  ProgUsedMemMB?: number;
  DiskUsedMB?: number;
  DiskFreeMB?: number;
  DiskCapMB?: number;
  SysLoad?: string;
  NumCPU?: number;
  NumGoMaxProcs?: number;
  NumGoroutines?: number;
  PID?: number;
  PPID?: number;
  UID?: number;
  EUID?: number;
  GID?: number;
  EGID?: number;
  ExePath?: string;
  CLIFlags?: string[];
  WorkingDirPath?: string;
  WorkingDirContent?: string[];
  EnvironmentVars?: string[];
}

export interface StatsDisplayValue {
  Lowest?: number;
  Average?: number;
  Highest?: number;
  Total?: number;
  Count?: number;
  Summary?: string;
}

export interface DaemonStatsDisplay extends StatsDisplayValue {
  Name?: string;
}

export interface ProgramStats {
  AutoUnlock?: StatsDisplayValue;
  DNSOverTCP?: StatsDisplayValue;
  DNSOverUDP?: StatsDisplayValue;
  HTTP?: StatsDisplayValue;
  HTTPProxy?: StatsDisplayValue;
  TCPOverDNS?: StatsDisplayValue;
  PlainSocketTCP?: StatsDisplayValue;
  PlainSocketUDP?: StatsDisplayValue;
  SimpleIPServiceTCP?: StatsDisplayValue;
  SimpleIPServiceUDP?: StatsDisplayValue;
  SMTP?: StatsDisplayValue;
  SockdTCP?: StatsDisplayValue;
  SockdUDP?: StatsDisplayValue;
  TelegramBot?: StatsDisplayValue;
  OutgoingMailBytes?: number;
}

export interface SystemInfo {
  Status?: ProgramStatusSummary;
  Stats?: ProgramStats;
}

@Injectable({ providedIn: 'root' })
export class LaitosClientService {
  constructor(readonly httpClient: HttpClient) {
  }

  getSystemInfo(server: Server): Observable<SystemInfo> {
    return this.httpClient.get<SystemInfo>(server.getInfoAddr(), { headers: { 'Accept': 'application/json' } });
  }
}
