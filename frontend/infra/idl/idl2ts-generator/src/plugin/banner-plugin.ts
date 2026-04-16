import { EOL } from 'os';

import { type IPlugin, type Program, before } from '@coze-arch/idl2ts-plugin';

import { type Contexts, HOOK } from '../context';

interface BannerOptions {
  banner?: string;
}

export class BannerPlugin implements IPlugin {
  private options: BannerOptions;

  constructor(options: BannerOptions) {
    this.options = options;
  }

  apply(program: Program<Contexts>): void {
    const { banner } = this.options;
    if (!banner) {
      return;
    }

    program.register(before(HOOK.WRITE_FILE), ctx => {
      ctx.content = banner + EOL + ctx.content;
      return ctx;
    });
  }
}
