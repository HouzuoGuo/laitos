import { KeyValue } from '@angular/common';
import { Component, OnDestroy, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, ReplaySubject, interval, shareReplay, switchMap, takeUntil } from 'rxjs';
import { ConfigService, Server } from './config.service';
import { LaitosClientService, SystemInfo } from './laitos.service';

@Component({
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.css',
})
export class DashboardComponent implements OnInit, OnDestroy {
  readonly destroyed: ReplaySubject<boolean> = new ReplaySubject(1);
  readonly systemInfo: Map<string, Observable<SystemInfo> | null | undefined>;

  readonly infoTableColumns = ['key', 'value'];

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

  ngOnInit() {
  }

  ngOnDestroy() {
    this.destroyed.next(true);
    this.destroyed.complete();
  }

  infoTableDataSource(info: SystemInfo): KeyValue<string, string>[] {
    const ret: Array<KeyValue<string, string>> = [];
    ret.push({ key: 'System clock', value: info.Status?.ClockTime ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Public IP', value: info.Status?.PublicIP ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Process ID', value: info.Status?.PID ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'User ID', value: info.Status?.UID ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Memory usage', value: info.Status?.ProgUsedMemMB ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'GOMAXPROCS', value: info.Status?.NumGoMaxProcs ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Goroutines', value: info.Status?.NumGoroutines ?? '' } as KeyValue<string, string>);
    ret.push({ key: 'Working directory', value: info.Status?.WorkingDirPath ?? '' } as KeyValue<string, string>);
    return ret;
  }

  navSetup() {
    this.router.navigate(['/setup']);
  }
}
