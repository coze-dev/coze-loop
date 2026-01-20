// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useConfigContext } from '@/config-provider';

export const useReport = () => {
  const {
    workspaceConfig,
    bizId,
    envConfig = {
      isOverSea: false,
      isDev: false,
    },
  } = useConfigContext();

  return (envName: string, params?: Record<string, string | number>) => {
    const { workspaceId = '' } = workspaceConfig ?? {};
    if (envConfig?.isDev || !window.teaCall || !workspaceId) {
      return;
    }
    window.teaCall(
      envName,
      {
        ...(params ?? {}),
        workspace_id: workspaceId,
        biz_id: bizId,
      },
      envConfig ?? {
        isOverSea: false,
        isDev: false,
      },
    );
  };
};
