// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { debounce, isEqual } from 'lodash-es';
import { SLEEP_TIME } from '@cozeloop/toolkit';
import { Role, TemplateType } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';

import { getPromptStorageInfo, setPromptStorageInfo } from '@/utils/prompt';
import { type PromptState, usePromptStore } from '@/store/use-prompt-store';
import {
  type PromptMockDataState,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { PromptStorageKey } from '@/consts';

import { mockInfo, mockMockSet } from '../consts/playground-mock';

type PlaygroundInfoStorage = Record<string, PromptState>;

type PlaygroundMockSetStorage = Record<string, PromptMockDataState>;

export const usePlayground = ({
  spaceID,
  promptID,
  useMockData,
}: {
  spaceID: string;
  promptID?: string;
  useMockData?: boolean;
}) => {
  const {
    setPromptInfo,
    setMessageList,
    setModelConfig,
    setToolCallConfig,
    setTools,
    setVariables,
    setCurrentModel,
    setTemplateType,
    clearStore: clearPromptStore,
  } = usePromptStore(
    useShallow(state => ({
      setPromptInfo: state.setPromptInfo,
      setMessageList: state.setMessageList,
      setModelConfig: state.setModelConfig,
      setToolCallConfig: state.setToolCallConfig,
      setTools: state.setTools,
      setVariables: state.setVariables,
      setCurrentModel: state.setCurrentModel,
      setTemplateType: state.setTemplateType,
      clearStore: state.clearStore,
    })),
  );
  const { setAutoSaving, clearBasicStore, setBasicReadonly } = useBasicStore(
    useShallow(state => ({
      setBasicReadonly: state.setReadonly,
      setAutoSaving: state.setAutoSaving,
      clearBasicStore: state.clearBasicStore,
    })),
  );
  const {
    setHistoricMessage,
    setMockVariables,
    setUserDebugConfig,
    clearMockdataStore,
    setCompareConfig,
  } = usePromptMockDataStore(
    useShallow(state => ({
      setHistoricMessage: state.setHistoricMessage,
      setMockVariables: state.setMockVariables,
      setUserDebugConfig: state.setUserDebugConfig,
      compareConfig: state.compareConfig,
      setCompareConfig: state.setCompareConfig,
      clearMockdataStore: state.clearMockdataStore,
    })),
  );

  const [initPlaygroundLoading, setInitPlaygroundLoading] = useState(true);

  const setPlaygroundInfo = (info: PromptState) => {
    setTools(info?.tools || []);
    setModelConfig(info?.modelConfig || {});
    setToolCallConfig(info?.toolCallConfig || {});
    setVariables(info?.variables || []);
    setMessageList(
      info?.messageList || [{ role: Role.System, content: '', key: nanoid() }],
    );
    setCurrentModel(info?.currentModel || {});
    setTemplateType(
      info?.templateType?.type
        ? info?.templateType
        : {
            type:
              (info?.templateType as unknown as TemplateType) ||
              TemplateType.Normal,
            value: TemplateType.Normal,
          },
    );
    setPromptInfo({
      workspace_id: spaceID,
      prompt_draft: { draft_info: {} },
    });

    setInitPlaygroundLoading(false);
  };

  useEffect(() => {
    setInitPlaygroundLoading(true);
    setBasicReadonly(useMockData);
    const storagePlaygroundInfo = getPromptStorageInfo<PlaygroundInfoStorage>(
      PromptStorageKey.PLAYGROUND_INFO,
    );
    const oldInfo = storagePlaygroundInfo?.[spaceID];

    const info: PromptState | undefined = useMockData ? mockInfo : oldInfo;

    const storagePlaygroundMockSet =
      getPromptStorageInfo<PlaygroundMockSetStorage>(
        PromptStorageKey.PLAYGROUND_MOCKSET,
      );

    const oldMock = storagePlaygroundMockSet?.[spaceID];
    const mock: PromptMockDataState | undefined = useMockData
      ? mockMockSet
      : oldMock;

    if (mock) {
      setHistoricMessage(mock?.historicMessage || []);
      setMockVariables(mock?.mockVariables || []);
      setUserDebugConfig(mock?.userDebugConfig || {});
      setCompareConfig(mock?.compareConfig || {});
    }

    if (promptID && info?.promptInfo?.id !== promptID) {
      StonePromptApi.GetPrompt({
        prompt_id: promptID,
        with_commit: true,
      })
        .then(res => {
          if (res?.prompt) {
            const currentPromptDetail = res.prompt?.prompt_commit;
            setCurrentModel({});
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
            setMessageList(
              messageList.map(item => ({ ...item, key: nanoid() })),
            );

            setModelConfig(currentPromptDetail?.detail?.model_config);
            setToolCallConfig(currentPromptDetail?.detail?.tool_call_config);
            setTools(currentPromptDetail?.detail?.tools);

            if (
              currentPromptDetail?.detail?.prompt_template?.template_type !==
              TemplateType.Normal
            ) {
              const variablesDefs =
                currentPromptDetail?.detail?.prompt_template?.variable_defs ||
                [];
              setVariables(variablesDefs);
            }
            setPromptInfo(res.prompt);
            setInitPlaygroundLoading(false);
          }
        })
        .catch(() => {
          setPlaygroundInfo(info || {});
        });
    } else {
      setPlaygroundInfo(info || {});
    }

    return () => {
      setInitPlaygroundLoading(true);
      setTimeout(() => {
        clearBasicStore();
        clearPromptStore();
        clearMockdataStore();
      }, 0);
    };
  }, [spaceID, promptID, useMockData]);

  const saveMockSet = debounce((mockSet: PromptMockDataState, sID: string) => {
    const storagePlaygroundMockSet =
      getPromptStorageInfo<PlaygroundMockSetStorage>(
        PromptStorageKey.PLAYGROUND_MOCKSET,
      );
    setPromptStorageInfo<PlaygroundMockSetStorage>(
      PromptStorageKey.PLAYGROUND_MOCKSET,
      { ...storagePlaygroundMockSet, [sID]: mockSet },
    );
    setAutoSaving(false);
  }, SLEEP_TIME);

  const saveInfo = debounce((info: PromptState, sID: string) => {
    const storagePlaygroundInfo = getPromptStorageInfo<PlaygroundInfoStorage>(
      PromptStorageKey.PLAYGROUND_INFO,
    );
    setPromptStorageInfo<PlaygroundInfoStorage>(
      PromptStorageKey.PLAYGROUND_INFO,
      {
        ...storagePlaygroundInfo,
        [sID]: info,
      },
    );

    setAutoSaving(false);
  }, SLEEP_TIME);

  useEffect(() => {
    const dataSub = usePromptStore.subscribe(
      state => ({
        toolCallConfig: state.toolCallConfig,
        variables: state.variables,
        modelConfig: state.modelConfig,
        tools: state.tools,
        messageList: state.messageList,
        promptInfo: state.promptInfo,
        currentModel: state.currentModel,
        templateType: state.templateType,
      }),
      val => {
        if (!initPlaygroundLoading) {
          const time = `${new Date().getTime()}`;
          setPromptInfo({
            ...val.promptInfo,
            prompt_draft: {
              draft_info: { updated_at: time },
            },
          });
          setAutoSaving(true);
          saveInfo(
            {
              ...val,
              promptInfo: {
                ...val.promptInfo,
                prompt_draft: {
                  draft_info: { updated_at: time },
                },
              },
            },
            spaceID,
          );
        }
      },
      {
        equalityFn: isEqual,
        fireImmediately: true, // 是否在第一次调用（初始化时）立刻执行
      },
    );
    const mockSub = usePromptMockDataStore.subscribe(
      state => ({
        historicMessage: state.historicMessage,
        userDebugConfig: state.userDebugConfig,
        mockVariables: state.mockVariables,
        compareConfig: state.compareConfig,
      }),
      val => {
        if (!initPlaygroundLoading) {
          setAutoSaving(true);
          saveMockSet(val, spaceID);
        }
      },
      {
        equalityFn: isEqual,
        fireImmediately: true, // 是否在第一次调用（初始化时）立刻执行
      },
    );

    return () => {
      dataSub?.();
      mockSub?.();
    };
  }, [initPlaygroundLoading, spaceID]);

  return {
    initPlaygroundLoading,
  };
};
