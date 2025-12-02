import {
  type CommonLogOptions,
  type BaseLoggerOptions,
  type LoggerReportClient,
} from '../types';
import { shouldCloseConsole } from '../console-disable';
import { Logger as RawLogger, type BaseLogger } from './core';
export type SetupKey = 'no-console';
export type SetupConfig = Record<SetupKey, unknown>;

export class Logger extends RawLogger {
  private registeredInstance: Logger[] = [];
  static setupConfig: SetupConfig | null = null;

  private setDisableConsole() {
    if (!Logger.setupConfig?.['no-console']) {
      return;
    }
    const disableConsole = shouldCloseConsole();
    this.disableConsole = disableConsole;
    if (this.persist) {
      this.persist.disableConsole = disableConsole;
    }
  }

  /**
   * @deprecated logger方法仅作控制台打印用，如需日志上报请使用`import { reporter } from '@coze-arch/logger'，具体规范：
   */
  addClient(client: LoggerReportClient): void {
    super.addClient(client);
  }

  /**
   * @deprecated 该方法已废弃，请统一使用`import { reporter } from '@coze-arch/logger'替换，具体规范：
   */
  persist: BaseLogger<CommonLogOptions> = this.persist;

  /**
   * Setup some attributes of config of logger at any time
   * @param setupConfig the config object needed to setup
   */
  setup(config: SetupConfig) {
    Logger.setupConfig = config;
  }

  createLoggerWith(options: BaseLoggerOptions): Logger {
    const logger = new Logger(this.resolveCloneParams(options));
    this.registeredInstance.push(logger);
    return logger;
  }

  info(payload: string | CommonLogOptions): void {
    this.setDisableConsole();
    super.info(payload);
  }

  success(payload: string | CommonLogOptions): void {
    this.setDisableConsole();
    super.success(payload);
  }

  warning(payload: string | CommonLogOptions): void {
    this.setDisableConsole();
    super.warning(payload);
  }

  error(payload: CommonLogOptions & { error: Error }): void {
    this.setDisableConsole();
    super.error(payload);
  }
}

const logger = new Logger({
  clients: [],
  ctx: {
    meta: {},
  },
});

export { logger };
