import { KeyValue } from '@angular/common';
import { Component, OnDestroy } from '@angular/core';
import { MatChipSelectionChange } from '@angular/material/chips';
import { Sort } from '@angular/material/sort';
import { Router } from '@angular/router';
import { Observable, ReplaySubject, interval, shareReplay, switchMap, takeUntil } from 'rxjs';
import { ConfigService, Server } from './config.service';
import { DaemonStatsDisplay, LaitosClientService, SystemInfo } from './laitos.service';

@Component({
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.css',
})
export class DashboardComponent implements OnDestroy {
  readonly CAPTION_COLUMN: Map<string, string> = new Map([
    ['Daemon stat', 'name'],
    ['Request count', 'count'],
    ['Fastest ms', 'lowest'],
    ['Average ms', 'average'],
    ['Slowest ms', 'highest'],
    ['Total sec', 'total'],
  ]);
  readonly COLUMN_CAPTION: Map<string, string> = new Map([...this.CAPTION_COLUMN.entries()].map(([caption, column]: [string, string]) => [column, caption]));

  readonly destroyed: ReplaySubject<boolean> = new ReplaySubject(1);
  readonly systemInfo: Map<string, Observable<SystemInfo>>;
  readonly procInfoTableColumns = ['key', 'value'];

  daemonStatsTableColumns = ['name', 'count'];
  daemonStatsSortColumn = 'name';
  daemonStatsSortDirection = 'asc';

  constructor(readonly router: Router, readonly configService: ConfigService, readonly laitosClient: LaitosClientService) {
    this.systemInfo = new Map(
      configService.read().servers.map((server: Server): [string, Observable<SystemInfo>] => {
        const fetch: Observable<SystemInfo> = interval(3000).pipe(
          takeUntil(this.destroyed),
          switchMap(_ => laitosClient.getSystemInfo(server)),
          shareReplay(1),
        );
        return [server.nickname, fetch];
      }));
  }

  ngOnDestroy() {
    this.destroyed.next(true);
    this.destroyed.complete();
  }

  procInfoTableDataSource(info: SystemInfo): KeyValue<string, string>[] {
    const ret: Array<KeyValue<string, string>> = [];
    ret.push({ key: 'System clock', value: info.Status?.ClockTime ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Public IP', value: info.Status?.PublicIP ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Process ID', value: info.Status?.PID ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'User ID', value: info.Status?.UID ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Memory usage', value: info.Status?.ProgUsedMemMB ?? '' + 'MB' } as KeyValue<string, string>);
    ret.push({ key: 'GOMAXPROCS', value: info.Status?.NumGoMaxProcs ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Goroutines', value: info.Status?.NumGoroutines ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Working directory', value: info.Status?.WorkingDirPath ?? '' } as KeyValue<string, string>);
    return ret;
  }

  statsTableDataSource(info: SystemInfo): Array<DaemonStatsDisplay> {
    const ret: Array<DaemonStatsDisplay> = [];
    ret.push({ Name: 'Auto unlock', ...info.Stats?.AutoUnlock } as DaemonStatsDisplay);
    ret.push({ Name: 'DNS TCP', ...info.Stats?.DNSOverTCP } as DaemonStatsDisplay);
    ret.push({ Name: 'DNS UDP', ...info.Stats?.DNSOverUDP } as DaemonStatsDisplay);
    ret.push({ Name: 'HTTP & HTTPS', ...info.Stats?.HTTP } as DaemonStatsDisplay);
    ret.push({ Name: 'HTTP proxy', ...info.Stats?.HTTPProxy } as DaemonStatsDisplay);
    ret.push({ Name: 'SMTP', ...info.Stats?.SMTP } as DaemonStatsDisplay);
    ret.push({ Name: 'Simple IP service TCP', ...info.Stats?.SimpleIPServiceTCP } as DaemonStatsDisplay);
    ret.push({ Name: 'Simple IP service UDP', ...info.Stats?.SimpleIPServiceUDP } as DaemonStatsDisplay);
    ret.push({ Name: 'TCP-over-DNS', ...info.Stats?.TCPOverDNS } as DaemonStatsDisplay);
    ret.push({ Name: 'Text command TCP', ...info.Stats?.PlainSocketTCP } as DaemonStatsDisplay);
    ret.push({ Name: 'Text command UDP', ...info.Stats?.PlainSocketUDP } as DaemonStatsDisplay);
    ret.push({ Name: 'Mail KB pending delivery', Count: info.Stats?.OutgoingMailBytes } as DaemonStatsDisplay);
    switch (this.daemonStatsSortColumn) {
      case 'name':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Name ?? '').localeCompare((b.Name ?? '')));
        } else {
          ret.sort((a, b) => (b.Name ?? '').localeCompare((a.Name ?? '')));
        }
        break;
      case 'count':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Count ?? 0) - (b.Count ?? 0));
        } else {
          ret.sort((a, b) => (b.Count ?? 0) - (a.Count ?? 0));
        }
        break;
      case 'highest':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Highest ?? 0) - (b.Highest ?? 0));
        } else {
          ret.sort((a, b) => (b.Highest ?? 0) - (a.Highest ?? 0));
        }
        break;
      case 'average':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Average ?? 0) - (b.Average ?? 0));
        } else {
          ret.sort((a, b) => (b.Average ?? 0) - (a.Average ?? 0));
        }
        break;
      case 'lowest':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Lowest ?? 0) - (b.Lowest ?? 0));
        } else {
          ret.sort((a, b) => (b.Lowest ?? 0) - (a.Lowest ?? 0));
        }
        break;
      case 'total':
        if (this.daemonStatsSortDirection == 'asc') {
          ret.sort((a, b) => (a.Total ?? 0) - (b.Total ?? 0));
        } else {
          ret.sort((a, b) => (b.Total ?? 0) - (a.Total ?? 0));
        }
        break;
    }
    return ret;
  }

  daemonStatsColumnChanged(column: string, ev: MatChipSelectionChange) {
    const ordered = [...this.CAPTION_COLUMN.values()];
    const selected = new Set([...this.daemonStatsTableColumns]);
    if (ev.selected) {
      selected.add(column);
    } else {
      selected.delete(column);
    }
    this.daemonStatsTableColumns = ordered.filter(col => selected.has(col));
  }

  statsTableSortChanged(ev: Sort) {
    this.daemonStatsSortColumn = ev.active;
    this.daemonStatsSortDirection = ev.direction;
  }

  navSetup() {
    this.router.navigate(['/setup']);
  }
}
