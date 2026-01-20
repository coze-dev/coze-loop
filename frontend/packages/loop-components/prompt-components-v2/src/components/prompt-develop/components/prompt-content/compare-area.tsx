// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable security/detect-object-injection */
import { useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { cloneDeep } from 'lodash-es';
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Message } from '@cozeloop/api-schema/prompt';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { CALL_SLEEP_TIME, EVENT_NAMES } from '@/consts';

import { SendMsgArea } from '../send-msg-area';
import { usePromptDevProviderContext } from '../prompt-provider';
import { CompareItem, type CompareItemRef } from '../compare-item';

export function CompareArea() {
  const { sendEvent, renerTipBanner } = usePromptDevProviderContext();
  const { streaming } = useBasicStore(
    useShallow(state => ({ streaming: state.streaming })),
  );
  const {
    setModelConfig,
    setTools,
    setToolCallConfig,
    setMessageList,
    promptInfo,
    setVariables,
    currentModel,
    setCurrentModel,
  } = usePromptStore(
    useShallow(state => ({
      setModelConfig: state.setModelConfig,
      setTools: state.setTools,
      setToolCallConfig: state.setToolCallConfig,
      setMessageList: state.setMessageList,
      promptInfo: state.promptInfo,
      setVariables: state.setVariables,
      currentModel: state.currentModel,
      setCurrentModel: state.setCurrentModel,
    })),
  );
  const {
    compareConfig,
    deleteComparePrompt,
    setModelConfigById,
    setToolsById,
    setMockVariablesById,
    setHistoricMessageById,
    setMessageListById,
    setToolCallConfigById,
    setHistoricMessage,
    setMockVariables,
    setMockToolsById,
    setVariablesById,
    setMockTools,
    setCurrentModelById,
  } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
      deleteComparePrompt: state.deleteComparePrompt,
      setModelConfigById: state.setModelConfigById,
      setVariablesById: state.setVariablesById,
      setToolsById: state.setToolsById,
      setMockVariablesById: state.setMockVariablesById,
      setHistoricMessageById: state.setHistoricMessageById,
      setMessageListById: state.setMessageListById,
      setToolCallConfigById: state.setToolCallConfigById,
      setHistoricMessage: state.setHistoricMessage,
      setMockVariables: state.setMockVariables,
      setMockToolsById: state.setMockToolsById,
      setMockTools: state.setMockTools,
      setCurrentModelById: state.setCurrentModelById,
    })),
  );

  const singleAreaRefs = useRef<CompareItemRef[] | null[]>([]);

  const compareStreaming = compareConfig?.groups?.some(it => it.streaming);
  const allStreaming = streaming || compareStreaming;

  const sendMessage = (message?: Message) => {
    singleAreaRefs?.current?.forEach((ref: CompareItemRef | null) => {
      ref?.sendMessage(message);
    });
  };

  const [exchanegKey, setExchangeKey] = useState(nanoid());
  const exchangePromptToDraft = (index: number) => {
    const {
      compareConfig: newComparePrompts,
      historicMessage = [],
      mockVariables = [],
      mockTools,
    } = usePromptMockDataStore.getState();
    const { modelConfig, messageList, tools, toolCallConfig, variables } =
      usePromptStore.getState();
    const compareItem = newComparePrompts?.groups?.[index];
    if (compareItem) {
      const newCompareItem = cloneDeep(compareItem);

      setVariablesById(index, variables);
      setToolsById(index, tools);
      setModelConfigById(index, modelConfig);
      setCurrentModelById(index, currentModel);

      setMockVariablesById(index, mockVariables);
      setMockToolsById(index, mockTools);
      setHistoricMessageById(index, historicMessage);
      setToolCallConfigById(index, toolCallConfig);

      setModelConfig(newCompareItem.prompt_detail?.model_config || {});
      setTools(newCompareItem.prompt_detail?.tools || []);
      setToolCallConfig(newCompareItem.prompt_detail?.tool_call_config);
      setHistoricMessage(newCompareItem.debug_core?.mock_contexts || []);
      setCurrentModel(newCompareItem.currentModel);

      setVariables(
        newCompareItem.prompt_detail?.prompt_template?.variable_defs,
      );
      setMockTools(newCompareItem.debug_core?.mock_tools || []);
      setMockVariables(newCompareItem.debug_core?.mock_variables || []);

      setTimeout(() => {
        setMessageListById(
          index,
          messageList?.map(it => ({ ...it, key: nanoid() })),
        );
        setMessageList(
          newCompareItem.prompt_detail?.prompt_template?.messages?.map(it => ({
            ...it,
            key: nanoid(),
          })),
        );
        setExchangeKey(nanoid());
      }, 0);
      sendEvent?.(EVENT_NAMES.prompt_compare_use, {
        prompt_id: `${promptInfo?.id || 'playground'}`,
        prompt_key: promptInfo?.prompt_key || 'playground',
      });
    }
  };
  const exchangePromptToCompare = useDebounceFn(exchangePromptToDraft, {
    wait: CALL_SLEEP_TIME,
  });

  return (
    <div className="flex flex-col h-full w-full" data-btm="c37617">
      <div
        className="flex flex-1 overflow-hidden w-full h-full flex-nowrap overflow-x-auto styled-scrollbar"
        style={{ scrollbarGutter: 'auto' }}
      >
        <CompareItem
          key={exchanegKey}
          ref={el => (singleAreaRefs.current[0] = el)}
          allStreaming={allStreaming}
          style={{ position: 'sticky', left: 0, zIndex: 7 }}
        />

        {compareConfig?.groups?.map((_, idx) => (
          <CompareItem
            key={`${idx}-${exchanegKey}`}
            uid={idx}
            title={`${I18n.t('control_group')} ${idx + 1}`}
            ref={el => (singleAreaRefs.current[idx + 1] = el)}
            deleteCompare={() => !allStreaming && deleteComparePrompt(idx)}
            exchangePromptToDraft={() =>
              !allStreaming && exchangePromptToCompare.run(idx)
            }
            allStreaming={allStreaming}
            canDelete={(compareConfig?.groups?.length || 0) > 1}
          />
        ))}
      </div>
      <div className="w-full flex-shrink-0 pl-6 pb-6 pt-3 border-0 border-t border-solid">
        {renerTipBanner?.(promptInfo)}
        <SendMsgArea onMessageSend={sendMessage} streaming={allStreaming} />
      </div>
    </div>
  );
}
