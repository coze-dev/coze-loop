// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-non-null-assertion */

import { useEffect } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { isEqual } from 'lodash-es';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { TemplateType } from '@cozeloop/api-schema/prompt';
import { promptDebug, promptManage } from '@cozeloop/api-schema';

import { messageId } from '@/utils/prompt';
import { type PromptState, usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  type PromptMockDataState,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';

interface UsePromptProps {
  promptID?: string;
  registerSub?: boolean;
}

export const usePrompt = ({
  promptID,
  registerSub = false,
}: UsePromptProps) => {
  const { spaceID } = useSpace();

  const { setReadonly, setSaveLock } = useBasicStore(
    useShallow(state => ({
      setReadonly: state.setReadonly,
      saveLock: state.saveLock,
      setSaveLock: state.setSaveLock,
    })),
  );

  const {
    setPromptInfo,
    setMessageList,
    setModelConfig,
    setToolCallConfig,
    setTools,
  } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      setPromptInfo: state.setPromptInfo,
      setMessageList: state.setMessageList,
      setModelConfig: state.setModelConfig,
      setToolCallConfig: state.setToolCallConfig,
      setTools: state.setTools,
      setVariables: state.setVariables,
    })),
  );
  const { setAutoSaving } = useBasicStore(
    useShallow(state => ({ setAutoSaving: state.setAutoSaving })),
  );
  const {
    setHistoricMessage,
    setMockVariables,
    setUserDebugConfig,
    setMockTools,
    setCompareConfig,
  } = usePromptMockDataStore(
    useShallow(state => ({
      setHistoricMessage: state.setHistoricMessage,
      setMockVariables: state.setMockVariables,
      setUserDebugConfig: state.setUserDebugConfig,
      setMockTools: state.setMockTools,
      setCompareConfig: state.setCompareConfig,
    })),
  );

  const { runAsync: getMockData } = useRequest(
    () =>
      promptDebug.GetDebugContext({
        workspace_id: spaceID,
        prompt_id: promptID!,
      }),
    {
      manual: true,
      ready: Boolean(spaceID && promptID),
    },
  );

  const { loading: getPromptLoading, runAsync: getPromptByVersion } =
    useRequest(
      (version?: string, withCommit?: boolean, onlyGetData?: boolean) => {
        setSaveLock(true);
        return promptManage
          .GetPrompt({
            prompt_id: promptID!,
            with_draft: !version,
            commit_version: version,
            with_commit: withCommit,
          })
          .then(async res => {
            if (onlyGetData) {
              return res;
            }
            setPromptInfo(res.prompt);
            const currentPromptDetail =
              res.prompt?.prompt_draft || res.prompt?.prompt_commit;

            const messageList =
              currentPromptDetail?.detail?.prompt_template?.messages || [];
            setMessageList(
              messageList.map(item => ({ ...item, key: nanoid() })),
            );

            setModelConfig(currentPromptDetail?.detail?.model_config);
            setToolCallConfig(currentPromptDetail?.detail?.tool_call_config);
            setTools(currentPromptDetail?.detail?.tools);
            setReadonly(Boolean(version));

            if (res.prompt) {
              const mockRes = await getMockData();
              const historicMessage: DebugMessage[] = (
                mockRes.debug_context?.debug_core?.mock_contexts || []
              )?.map((it: DebugMessage) => {
                const id = messageId();
                return {
                  id,
                  ...it,
                };
              });
              setHistoricMessage(historicMessage);
              setMockVariables(array =>
                array.map(it => {
                  const mock = (
                    mockRes.debug_context?.debug_core?.mock_variables || []
                  ).find(v => v.key === it.key);
                  return {
                    ...it,
                    value: mock?.value,
                  };
                }),
              );
              const mockTools =
                mockRes.debug_context?.debug_core?.mock_tools || [];
              setMockTools(mockTools);
              const userDebugConfig = mockRes.debug_context?.debug_config || {};
              setUserDebugConfig(userDebugConfig);
              setCompareConfig(mockRes.debug_context?.compare_config);
            }

            setTimeout(() => {
              setSaveLock(false);
            }, 500);

            return res;
          });
      },
      {
        ready: Boolean(promptID && spaceID),
        manual: true,
        debounceWait: 800,
        onSuccess: () => {
          setAutoSaving(false);
        },
      },
    );

  const { runAsync: runSavePrompt, loading: savePromptLoading } = useRequest(
    (params: PromptState & { mergeVersion?: string }) =>
      promptManage.SaveDraft({
        prompt_id: promptID!,
        prompt_draft: {
          detail: {
            prompt_template: {
              template_type: TemplateType.Normal,
              messages: params.messageList || [],
              variable_defs: params.variables,
            },
            tools: params.tools,
            tool_call_config: params.toolCallConfig,
            model_config: params.modelConfig,
          },
          draft_info: {
            ...params.promptInfo?.prompt_draft?.draft_info,
            base_version:
              params.promptInfo?.prompt_draft?.draft_info?.base_version ||
              params.promptInfo?.prompt_commit?.commit_info?.version,
          },
        },
      }),
    {
      manual: true,
      ready: Boolean(spaceID && promptID),
      debounceWait: 800,
      onError: err => {
        // TODO: 统一错误上报方法
        console.error(err);
      },
      onSuccess: res => {
        setPromptInfo(prev => ({
          ...prev,
          prompt_draft: {
            ...prev?.prompt_draft,
            draft_info: res.draft_info,
          },
        }));
      },
    },
  );

  const { run: runSaveMockInfo, loading: mockLoading } = useRequest(
    (params: PromptMockDataState) =>
      promptDebug.SaveDebugContext({
        workspace_id: spaceID!,
        prompt_id: promptID!,
        debug_context: {
          debug_core: {
            mock_contexts: params.historicMessage,
            mock_variables: params.mockVariables,
            mock_tools: params.mockTools,
          },
          debug_config: params.userDebugConfig,
          compare_config: params.compareConfig,
        },
      }),
    {
      manual: true,
      ready: Boolean(spaceID && promptID),
      debounceWait: 800,
    },
  );

  // 注册订阅
  useEffect(() => {
    let dataSub: () => void;
    let mockSub: () => void;
    if (registerSub && promptID) {
      dataSub = usePromptStore.subscribe(
        state => ({
          toolCallConfig: state.toolCallConfig,
          variables: state.variables,
          modelConfig: state.modelConfig,
          tools: state.tools,
          messageList: state.messageList,
        }),
        val => {
          const { readonly, saveLock } = useBasicStore.getState();
          const { promptInfo: currentPromptInfo } = usePromptStore.getState();
          if (!saveLock && currentPromptInfo?.id === promptID && !readonly) {
            // 调用 SavePrompt 接口
            runSavePrompt({ ...val, promptInfo: currentPromptInfo });
          }
          if (currentPromptInfo?.id && currentPromptInfo.id !== promptID) {
            console.error(I18n.t('prompt_id_inconsistent'));
          }
        },
        {
          equalityFn: isEqual,
          fireImmediately: true,
        },
      );
      mockSub = usePromptMockDataStore.subscribe(
        state => ({
          historicMessage: state.historicMessage,
          userDebugConfig: state.userDebugConfig,
          mockVariables: state.mockVariables,
          comparePrompts: state.compareConfig,
          mockTools: state.mockTools,
        }),
        val => {
          const { saveLock } = useBasicStore.getState();
          const { promptInfo: currentPromptInfo } = usePromptStore.getState();
          if (!saveLock && currentPromptInfo?.id === promptID) {
            const isCompare = val.comparePrompts?.groups?.length;

            // 调用 SaveMockInfo 接口
            runSaveMockInfo({
              mockVariables: val.mockVariables,
              historicMessage: val.historicMessage,
              mockTools: val.mockTools,
              userDebugConfig: val.userDebugConfig,
              compareConfig: isCompare ? val.comparePrompts : undefined,
            });
          }
        },
        {
          equalityFn: isEqual,
          fireImmediately: true,
        },
      );
    }
    return () => {
      dataSub?.();
      mockSub?.();
    };
  }, [registerSub, promptID]);

  useEffect(() => {
    setAutoSaving(savePromptLoading || mockLoading);
  }, [savePromptLoading, mockLoading]);

  return {
    getPromptLoading,
    getPromptByVersion,
  };
};
