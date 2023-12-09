import { ApplicationRef, ChangeDetectorRef, Component, OnInit } from '@angular/core';
import { AbstractControl, FormArray, FormBuilder, FormControl, FormGroup, ValidationErrors, ValidatorFn } from '@angular/forms';
import { Router } from '@angular/router';
import { of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { Config, ConfigService, Server } from './config.service';
import { LaitosClientService, SystemInfo } from './laitos.service';

const uniqueNickValidator: ValidatorFn = (control: AbstractControl): ValidationErrors | null => {
  const array = control as FormArray;
  const allNicks = [];
  for (let i = 0; i < array.length; i++) {
    const nick = array.at(i).value.nickname;
    if (nick) {
      allNicks.push(nick);
    }
  }
  if (new Set(allNicks).size !== array.length) {
    return { dupNickname: true };
  }
  return null;
}

@Component({
  templateUrl: './setup.component.html',
  styleUrl: './setup.component.css',
})
export class SetupComponent implements OnInit {
  readonly EMPTY_SERVER = new Server('', '', '');
  form: FormGroup;
  builder: FormBuilder;

  constructor(readonly configService: ConfigService, readonly laitosClient: LaitosClientService, readonly router: Router, readonly appRef: ApplicationRef, readonly detectRef: ChangeDetectorRef) {
    this.builder = new FormBuilder();
    this.form = this.builder.group({ servers: this.builder.array([], uniqueNickValidator) });
  }

  ngOnInit() {
    this.configService.read().servers.forEach((srv: Server) => {
      this.addServer(srv);
    });
  }

  getServers(): FormArray {
    return this.form.get('servers') as FormArray;
  }

  addServer(server: Server) {
    this.getServers().push(this.builder.group({
      nickname: new FormControl(server.nickname),
      address: new FormControl(server.address),
      infoEndpoint: new FormControl(server.infoEndpoint),
    }));
  }

  getServer(index: number): Server | undefined {
    const group = this.getServers().at(index);
    if (!group.valid) {
      return undefined;
    }
    return new Server(group.value.nickname, group.value.address, group.value.infoEndpoint);
  }

  testServer(index: number) {
    const server = this.getServer(index);
    if (!server) {
      alert('Please enter server details.');
      return;
    }
    this.laitosClient.getSystemInfo(server).pipe(
      map((resp: SystemInfo) => {
        if (!resp.Status?.ClockTime) {
          throw new Error('unexpected empty server response');
        }
        return true;
      }),
      catchError((err: Error) => of(err)),
    ).subscribe((result: boolean | Error) => {
      if (result === true) {
        alert('It works!');
      } else {
        alert('Error: ' + (result as Error).message);
      }
    });
  }

  deleteServer(index: number) {
    this.getServers().removeAt(index);
  }

  submit() {
    if (this.getServers().length === 0) {
      alert('Please enter server details.');
      return;
    }
    const config = new Config();
    for (let i = 0; i < this.getServers().length; i++) {
      const server = this.getServer(i);
      if (server) {
        config.servers.push(server);
      }
    }
    this.configService.update(config);
    this.router.navigate(['/dashboard']);
  }
}
