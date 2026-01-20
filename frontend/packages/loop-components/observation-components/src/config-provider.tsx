// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type PropsWithChildren } from 'react';

import { I18nProvider, type Locale } from './i18n/context';

export interface WorkspaceConfig {
  workspaceId: string | number;
  domain: string;
  token: string;
}

export interface EnvConfig {
  isOverSea: boolean;
  isDev: boolean;
}

export interface ConfigProviderProps {
  /** 这里用来覆盖原本的 coze-design 的 CSS 变量 */
  theme?: Record<string, unknown>;
  /** 这里用来注册时区 */
  timeZone?: string;
  /** 国际化语言设置 */
  locale?: Locale;
  /** 业务自定义埋点 */
  sendEvent?: (name: string, params: Record<string, unknown>) => void;
  /** 工作空间配置 */
  workspaceConfig?: WorkspaceConfig;
  /** 环境配置 */
  envConfig?: EnvConfig;
  /** 业务 ID !!!不要使用 cozeloop 和 fornax 组件内部对本业务有特殊处理 */
  bizId?: string;
}

const ConfigContext = React.createContext<ConfigProviderProps>({
  envConfig: {
    isOverSea: false,
    isDev: false,
  },
  workspaceConfig: {
    workspaceId: '',
    domain: '',
    token: '',
  },
  bizId: '',
});

export const ConfigProvider: React.FC<
  PropsWithChildren<ConfigProviderProps>
> = props => {
  const {
    theme,
    timeZone,
    locale,
    children,
    sendEvent,
    workspaceConfig,
    envConfig,
    bizId,
  } = props;

  return (
    <ConfigContext.Provider
      value={{
        theme,
        timeZone,
        locale,
        sendEvent,
        workspaceConfig,
        envConfig,
        bizId,
      }}
    >
      <div
        id="cozeloop-observation-components"
        className="w-full h-full max-w-full overflow-hidden"
      >
        <I18nProvider defaultLocale={locale} children={children} />
      </div>
    </ConfigContext.Provider>
  );
};

export const useConfigContext = () => React.useContext(ConfigContext);
