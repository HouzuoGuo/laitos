import { Component } from '@angular/core';
import { FormControl, FormGroup } from '@angular/forms';
import { Router } from '@angular/router';
import { of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { Config, ConfigService } from './config.service';
import { LaitosClientService, SystemInfo } from './laitos.service';

@Component({
  templateUrl: './setup.component.html',
  styleUrl: './setup.component.css',
})
export class SetupComponent {
  conf: Config;
  configForm: FormGroup;

  constructor(readonly configService: ConfigService, readonly laitosClient: LaitosClientService, readonly router: Router) {
    this.conf = configService.read();
    this.configForm = new FormGroup({
      serverAddr: new FormControl(this.conf.serverAddr),
      infoEndpoint: new FormControl(this.conf.infoEndpoint),
    });
  }

  submit() {
    const newConf = new Config();
    Object.assign(newConf, this.configForm.value);
    this.configService.update(newConf);

    this.laitosClient.getSystemInfo().pipe(
      map((resp: SystemInfo) => {
        if (!resp.Status?.ClockTime) {
          throw new Error('unexpected empty response');
        }
        return true;
      }),
      catchError((err: Error) => of(err)),
    ).subscribe((result: boolean | Error) => {
      if (result === true) {
        alert('It works!');
        this.router.navigate(['dashboard']);
      } else {
        alert('Error: ' + (result as Error).message);
      }
    });
  }
}
