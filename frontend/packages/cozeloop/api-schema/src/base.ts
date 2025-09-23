export interface TrafficEnv {
  Open: boolean,
  Env: string,
}
export interface Base {
  LogID: string,
  Caller: string,
  Addr: string,
  Client: string,
  TrafficEnv?: TrafficEnv,
  Extra?: {
    [key: string | number]: string
  },
}
export interface BaseResp {
  StatusMessage: string,
  StatusCode: number,
  Extra?: {
    [key: string | number]: string
  },
}