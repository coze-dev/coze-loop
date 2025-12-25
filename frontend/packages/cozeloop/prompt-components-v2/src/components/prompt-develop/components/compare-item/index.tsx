// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import {
  type CSSProperties,
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';

import { useShallow } from 'zustand/react/shallow';
import { Resizable } from 're-resizable';
import { isUndefined } from 'lodash-es';
import { useSize } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type DebugMessage,
  type DebugToolCall,
  type Message,
  Role,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozDubbleHorizontal,
  IconCozSetting,
  IconCozStopCircle,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  IconButton,
  Toast,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import {
  convertMultimodalMessage,
  getPlaceholderErrorContent,
  messageId,
} from '@/utils/prompt';
import {
  createLLMRun,
  PromptExecuteStatus,
  sendPromptExecuteEvent,
} from '@/utils/llm';
import { useBasicStore } from '@/store/use-basic-store';
import { isResponding, useLLMStreamRun } from '@/hooks/use-llm-stream-run';
import { useCompare } from '@/hooks/use-compare';
import { type ModelConfigWithName } from '@/components/model-config-editor/utils';
import { PopoverModelConfigEditor } from '@/components/model-config-editor/popover-model-config-editor';

import { VariablesCard } from '../variables-card';
import { ToolsCard } from '../tools-card';
import { usePromptDevProviderContext } from '../prompt-provider';
import { PromptEditorCard } from '../prompt-editor-card';
import { CompareMessageArea } from '../message-area';

import styles from './index.module.less';

interface CompareItemProps {
  uid?: number;
  title?: string;
  deleteCompare?: () => void;
  exchangePromptToDraft?: () => void;
  allStreaming?: boolean;
  style?: CSSProperties;
  canDelete?: boolean;
}

export interface CompareItemRef {
  sendMessage: (message?: Message) => void;
}

export const CompareItem = forwardRef<CompareItemRef, CompareItemProps>(
  (
    {
      uid,
      title,
      deleteCompare,
      exchangePromptToDraft,
      allStreaming,
      style,
      canDelete,
    },
    ref,
  ) => {
    const { sendEvent, modelInfo } = usePromptDevProviderContext();
    const warpperRef = useRef<HTMLDivElement>(null);
    const { readonly } = useBasicStore(
      useShallow(state => ({ readonly: state.readonly })),
    );

    const {
      streaming,
      messageList,
      variables,
      modelConfig,
      setModelConfig,
      historicMessage = [],
      setHistoricMessage,
      setStreaming,
      setCurrentModel,
    } = useCompare(uid);

    const size = useSize(warpperRef.current);
    const maxHeight = size?.height ? size.height - 40 : '100%';

    const [toolCalls, setToolCalls] = useState<DebugToolCall[]>([]);

    const {
      startStream,
      smoothExecuteResult,
      abort,
      stepDebuggingTrace,
      respondingStatus,
      reasoningContentResult,
    } = useLLMStreamRun(uid, () =>
      sendPromptExecuteEvent(PromptExecuteStatus.Canceled, sendEvent),
    );

    const runLLM = (
      queryMsg?: Message,
      history?: DebugMessage[],
      traceKey?: string,
    ) => {
      const placeholderHasError = messageList?.some(message => {
        if (message.role === Role.Placeholder) {
          return Boolean(getPlaceholderErrorContent(message, variables));
        }
        return false;
      });
      if (placeholderHasError) {
        return Toast.error(I18n.t('placeholder_var_error'));
      }

      setStreaming?.(true);

      createLLMRun({
        uid,
        history,
        startStream,
        message: queryMsg,
        traceKey,
        notReport: !uid,
        singleRound: false,
        sendEvent,
      });
    };

    const lastIndex = historicMessage.length - 1;

    const rerunSendMessage = () => {
      const history = historicMessage.slice(0, lastIndex);
      const lastContent = historicMessage?.[lastIndex - 1];
      const last = lastContent;

      const chatArray = history.filter(v => Boolean(v)) as Message[];

      const historyHasEmpty = Boolean(
        chatArray.length &&
          chatArray.some(it => {
            if (it?.parts?.length) {
              return false;
            }
            return !it?.content && !it.tool_calls?.length;
          }),
      );

      if (historyHasEmpty) {
        return Toast.error(I18n.t('historical_data_has_empty_content'));
      }

      setHistoricMessage?.(history);
      const newHistory = historicMessage
        .slice(0, lastIndex - 1)
        .map(it => ({
          id: it.id,
          role: it?.role,
          content: it?.content,
          parts: it?.parts,
        }))
        .filter(v => Boolean(v));

      runLLM(
        last
          ? { content: last.content, role: last.role, parts: last.parts }
          : undefined,
        newHistory,
      );
    };

    const stopStreaming = () => {
      abort();
      if (streaming) {
        setHistoricMessage?.(list => [
          ...(list || []),
          {
            isEdit: false,
            id: messageId(),
            role: Role.Assistant,
            content: smoothExecuteResult,
            tool_calls: toolCalls,
          },
        ]);
      }
    };

    const sendMessage = (message?: Message) => {
      if (
        !messageList?.length &&
        !(message?.content || message?.parts?.length)
      ) {
        Toast.error(I18n.t('add_prompt_tpl_or_input_question'));
        return;
      }
      const chatArray = historicMessage.filter(v => Boolean(v)) as Message[];
      const historyHasEmpty = Boolean(
        chatArray.length &&
          chatArray.some(it => {
            if (it?.parts?.length) {
              return false;
            }
            return !it?.content && !it.tool_calls?.length;
          }),
      );

      if (message?.content || message?.parts?.length) {
        if (historyHasEmpty) {
          return Toast.error(I18n.t('historical_data_has_empty_content'));
        }

        if (message) {
          const newMessage = convertMultimodalMessage(message);
          setHistoricMessage?.(list => [
            ...(list || []),
            {
              isEdit: false,
              id: messageId(),
              content: newMessage.content,
              role: newMessage.role,
              parts: newMessage.parts,
            },
          ]);
        }

        const history = chatArray.map(it => ({
          role: it.role,
          content: it.content,
          parts: it.parts,
        }));
        runLLM(message, history);
      } else if (chatArray.length) {
        const last = chatArray?.[chatArray.length - 1];
        if (last.role === Role.Assistant) {
          rerunSendMessage();
        } else {
          if (historyHasEmpty && chatArray.length > 2) {
            return Toast.error(I18n.t('historical_data_has_empty_content'));
          }
          const history = chatArray.slice(0, chatArray.length - 1).map(it => ({
            role: it.role,
            content: it.content,
            parts: it.parts,
          }));
          runLLM(last, history);
        }
      } else {
        runLLM(undefined, []);
      }
    };

    useImperativeHandle(ref, () => ({
      sendMessage,
    }));

    useEffect(() => {
      if (!isResponding(respondingStatus)) {
        setStreaming?.(false);
        setToolCalls?.([]);
      }
    }, [respondingStatus, stepDebuggingTrace]);

    return (
      <div className={styles['compare-item']} ref={warpperRef} style={style}>
        <div className="flex flex-1 flex-col w-full min-h-[40px]">
          <div
            className="px-6 py-2 box-border border-0 border-t border-b border-solid coz-fg-plus w-full h-[40px] flex items-center justify-between"
            style={{ background: '#F6F6FB' }}
          >
            <div className="flex items-center gap-2 flex-shrink-0">
              <Typography.Text className="flex-shrink-0" strong>
                {title || I18n.t('benchmark_group')}
              </Typography.Text>
              {!isUndefined(uid) ? (
                <div className={styles['btn-group']}>
                  <Tooltip
                    content={I18n.t('set_to_reference_group')}
                    theme="dark"
                  >
                    <IconButton
                      color="secondary"
                      size="small"
                      icon={<IconCozDubbleHorizontal />}
                      onClick={exchangePromptToDraft}
                      disabled={allStreaming}
                    />
                  </Tooltip>
                  <Tooltip
                    content={I18n.t('delete_control_group')}
                    theme="dark"
                  >
                    <IconButton
                      color="secondary"
                      size="small"
                      icon={<IconCozTrashCan />}
                      onClick={deleteCompare}
                      disabled={allStreaming || !canDelete}
                    />
                  </Tooltip>
                </div>
              ) : null}
            </div>
            <PopoverModelConfigEditor
              key={uid}
              value={modelConfig as ModelConfigWithName}
              onChange={config => {
                setModelConfig({ ...config });
              }}
              disabled={streaming || readonly}
              renderDisplayContent={model => (
                <Button color="secondary">
                  <Typography.Text
                    className="!max-w-[160px]"
                    ellipsis={{ showTooltip: true }}
                  >
                    {model?.name}
                  </Typography.Text>
                  <IconCozSetting className="ml-4" />
                </Button>
              )}
              models={modelInfo?.list || []}
              onModelChange={setCurrentModel}
              modelSelectProps={{
                className: 'w-full',
                loading: modelInfo?.loading,
              }}
            />
          </div>
          <div className="flex-1 px-6 pr-[18px] py-3 flex flex-col gap-3  overflow-y-auto styled-scrollbar">
            <PromptEditorCard canCollapse uid={uid} />
            <Divider />
            <VariablesCard uid={uid} />
            <Divider />
            <ToolsCard uid={uid} />
          </div>
        </div>
        <Resizable
          enable={{ top: true }}
          handleComponent={{
            top: (
              <div className="h-[5px] mt-[5px] border-0 border-solid border-brand-9 hover:border-t-2"></div>
            ),
          }}
          className="w-full overflow-x-hidden flex flex-col"
          minHeight="40px"
          maxHeight={maxHeight}
          defaultSize={{
            width: '100%',
            height: '52%',
          }}
        >
          <div
            className="px-6 py-2 box-border border-0 border-t border-b border-solid coz-fg-plus w-full h-[40px]"
            style={{ background: '#F6F6FB' }}
          >
            <Typography.Text strong>
              {I18n.t('preview_and_debug')}
            </Typography.Text>
          </div>
          <CompareMessageArea
            uid={uid}
            className="!px-6 !py-2"
            streaming={streaming}
            streamingMessage={smoothExecuteResult}
            toolCalls={toolCalls}
            reasoningContentResult={reasoningContentResult}
            rerunLLM={rerunSendMessage}
            stepDebuggingTrace={stepDebuggingTrace}
            setToolCalls={setToolCalls}
          />
        </Resizable>
        <div className="flex items-center justify-center flex-shrink-0 pb-2 h-[28px]">
          {streaming ? (
            <Button
              color="primary"
              theme="light"
              icon={<IconCozStopCircle />}
              size="small"
              onClick={stopStreaming}
            >
              {I18n.t('stop_respond')}
            </Button>
          ) : null}
        </div>
      </div>
    );
  },
);
