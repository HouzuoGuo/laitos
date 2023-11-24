import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { ConfigService } from './config.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
})
export class AppComponent implements OnInit {
  constructor(
    readonly configService: ConfigService,
    readonly router: Router,
  ) {}

  ngOnInit() {
    if (!this.configService.read().isPresent()) {
      this.router.navigate(['/setup']);
    }
  }
}
