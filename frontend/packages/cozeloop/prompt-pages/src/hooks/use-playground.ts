// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { debounce, isEqual } from 'lodash-es';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Message, Role, TemplateType } from '@cozeloop/api-schema/prompt';

import { getPromptStorageInfo, setPromptStorageInfo } from '@/utils/prompt';
import { type PromptState, usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  type PromptMockDataState,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import {
  CALL_SLEEP_TIME,
  MESSAGE_TYPE_MAP,
  MessageType,
  PromptStorageKey,
} from '@/consts';

import { mockInfo, mockMockSet } from './playground-mock';

type PlaygroundInfoStorage = Record<string, PromptState>;

type PlaygroundMockSetStorage = Record<string, PromptMockDataState>;

export const usePlayground = () => {
  const globalDisabled = useGuard({ point: GuardPoint['pe.prompt.global'] });
  const { spaceID } = useSpace();
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
  const { setAutoSaving, clearStore: clearBasicStore } = useBasicStore(
    useShallow(state => ({
      setAutoSaving: state.setAutoSaving,
      clearStore: state.clearStore,
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

  useEffect(() => {
    setInitPlaygroundLoading(true);
    const storagePlaygroundInfo = getPromptStorageInfo<PlaygroundInfoStorage>(
      PromptStorageKey.PLAYGROUND_INFO,
    );
    const oldInfo = storagePlaygroundInfo?.[spaceID];

    const info: PromptState | undefined = globalDisabled.data.readonly
      ? mockInfo
      : oldInfo;

    const storagePlaygroundMockSet =
      getPromptStorageInfo<PlaygroundMockSetStorage>(
        PromptStorageKey.PLAYGROUND_MOCKSET,
      );
    const oldMock = storagePlaygroundMockSet?.[spaceID];
    const mock: PromptMockDataState | undefined = globalDisabled.data.readonly
      ? mockMockSet
      : oldMock;

    if (mock) {
      setHistoricMessage(
        mock?.historicMessage?.map(
          (it: DebugMessage & { message_type?: MessageType }) => ({
            ...it,
            role: MESSAGE_TYPE_MAP[it?.message_type ?? MessageType.Assistant],
          }),
        ) || [],
      );
      setMockVariables(mock?.mockVariables || []);
      setUserDebugConfig(mock?.userDebugConfig || {});
      setCompareConfig(mock?.compareConfig || {});
    }

    setTools(info?.tools || []);
    setModelConfig(info?.modelConfig || {});
    setToolCallConfig(info?.toolCallConfig || {});
    setVariables(info?.variables || []);
    setMessageList(
      info?.messageList?.map(
        (it: Message & { message_type?: MessageType }) => ({
          ...it,
          role: MESSAGE_TYPE_MAP[it?.message_type ?? MessageType.System],
        }),
      ) || [{ role: Role.System, content: '', key: nanoid() }],
    );
    setCurrentModel(info?.currentModel || {});
    setTemplateType(info?.templateType || TemplateType.Normal);
    setPromptInfo({
      workspace_id: spaceID,
      prompt_draft: { draft_info: {} },
    });

    setInitPlaygroundLoading(false);

    return () => {
      setInitPlaygroundLoading(true);
      setTimeout(() => {
        clearPromptStore();
        clearBasicStore();
        clearMockdataStore();
      }, 0);
    };
  }, [spaceID]);

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
  }, CALL_SLEEP_TIME);

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
  }, CALL_SLEEP_TIME);

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
