import { Injectable } from '@angular/core';

const KEY_SERVER_ADDR = 'laitos-cp-serveraddr';

const KEY_INFO_ENDPOINT = 'laitos-cp-infoendpoint';

export class Config {
  constructor(public serverAddr: string = '', public infoEndpoint: string = '') {
  }

  isPresent(): boolean {
    return !!this.serverAddr;
  }

  getInfoAddr(): string {
    return this.serverAddr + this.infoEndpoint;
  }
}

@Injectable({ providedIn: 'root' })
export class ConfigService {
  constructor() {}

  read(): Config {
    return new Config(window.localStorage.getItem(KEY_SERVER_ADDR) || '', window.localStorage.getItem(KEY_INFO_ENDPOINT) || '');
  }

  update(config: Config) {
    window.localStorage.setItem(KEY_SERVER_ADDR, config.serverAddr);
    window.localStorage.setItem(KEY_INFO_ENDPOINT, config.infoEndpoint);
  }
}
