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

  navSettings() {
    this.router.navigate(['/setup']);
  }
}
