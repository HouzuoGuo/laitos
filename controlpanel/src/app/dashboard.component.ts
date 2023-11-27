import { Component, OnDestroy, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, ReplaySubject, interval, shareReplay, switchMap, takeUntil } from 'rxjs';
import { LaitosClientService, SystemInfo } from './laitos.service';

@Component({
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.css',
})
export class DashboardComponent implements OnInit, OnDestroy {
  readonly destroyed: ReplaySubject<boolean> = new ReplaySubject(1);
  readonly systemInfo: Observable<SystemInfo>;

  constructor(readonly router: Router, readonly laitosClient: LaitosClientService) {
    this.systemInfo = interval(3000).pipe(
      takeUntil(this.destroyed),
      switchMap(_ => laitosClient.getSystemInfo()),
      shareReplay(1),
    );
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
