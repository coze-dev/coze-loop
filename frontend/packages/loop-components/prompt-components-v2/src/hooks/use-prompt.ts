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
import { SLEEP_TIME } from '@cozeloop/toolkit';
import { TemplateType, ToolChoiceType } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';

import {
  convertSnippetsToMap,
  messageId,
  messagesHasSnippet,
  variablesAddSourceMap,
} from '@/utils/prompt';
import { type PromptState, usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  type PromptMockDataState,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';

interface UsePromptProps {
  spaceID: string;
  promptID?: string;
  regiesterSub?: boolean;
}

export const usePrompt = ({
  spaceID,
  promptID,
  regiesterSub = false,
}: UsePromptProps) => {
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
    setTemplateType,
    setVariables,
    setTotalReferenceCount,
    setSnippetMap,
  } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      setPromptInfo: state.setPromptInfo,
      setMessageList: state.setMessageList,
      setModelConfig: state.setModelConfig,
      setToolCallConfig: state.setToolCallConfig,
      setTools: state.setTools,
      setVariables: state.setVariables,
      setTemplateType: state.setTemplateType,
      setTotalReferenceCount: state.setTotalReferenceCount,
      setSnippetMap: state.setSnippetMap,
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

  const mockDataService = useRequest(
    () =>
      StonePromptApi.GetDebugContext({
        workspace_id: spaceID,
        prompt_id: promptID!,
      }),
    {
      manual: true,
      ready: Boolean(spaceID && promptID),
    },
  );

  const promptByVersionService = useRequest(
    ({
      version,
      withCommit = false,
      onlyGetData = false,
    }: {
      version?: string;
      withCommit?: boolean;
      onlyGetData?: boolean;
    }) => {
      setSaveLock(true);
      return StonePromptApi.GetPrompt({
        prompt_id: promptID!,
        with_draft: !version,
        with_default_config: !version,
        commit_version: version,
        with_commit: withCommit,
        workspace_id: spaceID!,
      }).then(async res => {
        setPromptInfo(res.prompt);
        setTotalReferenceCount(res.total_parent_references ?? 0);
        const currentPromptDetail = res.prompt?.prompt_draft ||
          res.prompt?.prompt_commit || { detail: res.default_config };

        setTemplateType({
          type:
            currentPromptDetail?.detail?.prompt_template?.template_type ||
            TemplateType.Normal,
          value:
            currentPromptDetail?.detail?.prompt_template?.template_type ||
            TemplateType.Normal,
        });

        const messageList =
          currentPromptDetail?.detail?.prompt_template?.messages || [];

        setModelConfig(currentPromptDetail?.detail?.model_config);
        setToolCallConfig(
          currentPromptDetail?.detail?.tool_call_config || {
            tool_choice: ToolChoiceType.Auto,
          },
        );
        setTools(currentPromptDetail?.detail?.tools);

        setReadonly(Boolean(version));

        if (res.prompt && !onlyGetData) {
          const mockRes = await mockDataService.runAsync();
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

          const snippets =
            currentPromptDetail?.detail?.prompt_template?.snippets || [];
          if (snippets.length) {
            setSnippetMap(map => ({
              ...map,
              ...convertSnippetsToMap(snippets),
            }));
          }

          const variablesDefs =
            currentPromptDetail?.detail?.prompt_template?.variable_defs || [];

          variablesAddSourceMap(messageList, variablesDefs || []);

          setVariables(variablesDefs);
          setMessageList(messageList.map(item => ({ ...item, key: nanoid() })));
          if (
            currentPromptDetail?.detail?.prompt_template?.template_type !==
            TemplateType.Normal
          ) {
            const mockVariables = (variablesDefs || []).map(it => {
              const mock = (
                mockRes.debug_context?.debug_core?.mock_variables || []
              ).find(v => v.key === it.key);
              return {
                ...it,
                ...mock,
              };
            });

            setMockVariables(mockVariables);
          } else {
            setMockVariables(array =>
              array.map(it => {
                const mock = (
                  mockRes.debug_context?.debug_core?.mock_variables || []
                ).find(v => v.key === it.key);
                return {
                  ...it,
                  value: mock?.value,
                  multi_part_values: mock?.multi_part_values,
                  placeholder_messages: mock?.placeholder_messages,
                };
              }),
            );
          }

          const mockTools = mockRes.debug_context?.debug_core?.mock_tools || [];
          setMockTools(mockTools);
          const userDebugConfig = mockRes.debug_context?.debug_config || {};
          setUserDebugConfig(userDebugConfig);
          setCompareConfig(mockRes.debug_context?.compare_config);
        }

        setTimeout(() => {
          setSaveLock(false);
        }, SLEEP_TIME);

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

  const savePromptService = useRequest(
    (params: PromptState & { mergeVersion?: string }) => {
      variablesAddSourceMap(params.messageList || [], params.variables || []);
      const hasSnippet = messagesHasSnippet(params.messageList || []);
      const promptDetail = {
        prompt_template: {
          template_type: (params.templateType?.value ??
            TemplateType.Normal) as TemplateType,
          messages: params.messageList || [],
          variable_defs: params.variables,
          has_snippet: hasSnippet,
        },
        tools: params.tools,
        tool_call_config: params.toolCallConfig,
        model_config: params.modelConfig,
      };
      return StonePromptApi.SaveDraft({
        prompt_id: promptID!,
        prompt_draft: {
          detail: promptDetail,
          draft_info: {
            ...params.promptInfo?.prompt_draft?.draft_info,
            base_version:
              params.promptInfo?.prompt_draft?.draft_info?.base_version ||
              params.promptInfo?.prompt_commit?.commit_info?.version,
          },
        },
      }).then(res => ({ ...res, detail: promptDetail }));
    },
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
            detail: { ...prev?.prompt_draft?.detail, ...res.detail },
            draft_info: {
              ...prev?.prompt_draft?.draft_info,
              ...res.draft_info,
            },
          },
        }));
      },
    },
  );

  const saveMockInfoService = useRequest(
    (params: PromptMockDataState) =>
      StonePromptApi.SaveDebugContext({
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
    if (regiesterSub && promptID) {
      dataSub = usePromptStore.subscribe(
        state => ({
          toolCallConfig: state.toolCallConfig,
          variables: state.variables,
          modelConfig: state.modelConfig,
          tools: state.tools,
          messageList: state.messageList,
          templateType: state.templateType,
        }),
        val => {
          const { readonly, saveLock } = useBasicStore.getState();
          const { promptInfo: currentPromptInfo } = usePromptStore.getState();
          if (!saveLock && currentPromptInfo?.id === promptID && !readonly) {
            // 调用 SavePrompt 接口
            savePromptService.run({
              ...val,
              promptInfo: currentPromptInfo,
            });
          }
          if (currentPromptInfo?.id && currentPromptInfo.id !== promptID) {
            console.error('promptID 不一致');
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
            saveMockInfoService.run({
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
  }, [regiesterSub, promptID]);

  useEffect(() => {
    setAutoSaving(savePromptService.loading || saveMockInfoService.loading);
  }, [savePromptService.loading, saveMockInfoService.loading]);

  return {
    promptByVersionService,
    savePromptService,
  };
};
