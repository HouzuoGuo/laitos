import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';

@Component({
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.css',
})
export class DashboardComponent implements OnInit {
  constructor(readonly router: Router) {}

  ngOnInit() {
  }

  navSettings() {
    this.router.navigate(['/setup']);
  }
}
