import { Injectable } from '@angular/core';

const KEY_SERVE_LEN = 'laitos-cp-serverlen';
const KEY_SERVER_NICKNAME = 'laitos-cp-servernick';
const KEY_SERVER_ADDRESS = 'laitos-cp-serveraddr';
const KEY_INFO_ENDPOINT = 'laitos-cp-infoendpoint';

export class Server {
  constructor(public nickname: string = '', public address: string = '', public infoEndpoint: string = '') {
    if (address && address.endsWith('/')) {
      this.address = address.substring(0, address.length - 1);
    }
    if (infoEndpoint && !infoEndpoint.startsWith('/')) {
      this.infoEndpoint = infoEndpoint.substring(1);
    }
  }
  getInfoAddr(): string {
    return this.address + this.infoEndpoint;
  }
}

export class Config {
  constructor(public servers: Server[] = []) {
  }

  isPresent(): boolean {
    return this.servers.some(s => s.getInfoAddr() !== '');
  }
}

@Injectable({ providedIn: 'root' })
export class ConfigService {
  constructor() {}

  read(): Config {
    const servers: Server[] = [];
    for (let i = 0; i < Number(window.localStorage.getItem(KEY_SERVE_LEN) || 0); i++) {
      const nickname = window.localStorage.getItem(KEY_SERVER_NICKNAME + i) || ''
      const address = window.localStorage.getItem(KEY_SERVER_ADDRESS + i) || ''
      const infoEndpoint = window.localStorage.getItem(KEY_INFO_ENDPOINT + i) || ''
      servers.push(new Server(nickname, address, infoEndpoint));
    }
    return new Config(servers);
  }

  update(config: Config) {
    window.localStorage.setItem(KEY_SERVE_LEN, config.servers.length + '');
    for (const index in config.servers) {
      window.localStorage.setItem(KEY_SERVER_NICKNAME + index, config.servers[index].nickname);
      window.localStorage.setItem(KEY_SERVER_ADDRESS + index, config.servers[index].address);
      window.localStorage.setItem(KEY_INFO_ENDPOINT + index, config.servers[index].infoEndpoint);
    }
  }
}
