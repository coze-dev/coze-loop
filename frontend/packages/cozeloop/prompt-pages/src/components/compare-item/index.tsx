/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import React, {
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
import {
  getPlaceholderErrorContent,
  PopoverModelConfigEditor,
} from '@cozeloop/prompt-components';
import { useModelList, useSpace } from '@cozeloop/biz-hooks-adapter';
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

import { convertMultimodalMessage, messageId } from '@/utils/prompt';
import { createLLMRun } from '@/utils/llm';
import { useBasicStore } from '@/store/use-basic-store';
import { isResponding, useLLMStreamRun } from '@/hooks/use-llm-stream-run';
import { useCompare } from '@/hooks/use-compare';

import { VariablesCard } from '../variables-card';
import { ToolsCard } from '../tools-card';
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
    const warpperRef = useRef<HTMLDivElement>(null);
    const { spaceIDWhenDemoSpaceItsPersonal } = useSpace();
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
    } = useLLMStreamRun(uid);

    const service = useModelList(spaceIDWhenDemoSpaceItsPersonal);

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
        return Toast.error('Placeholder 变量不存在或命名错误');
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
        return Toast.error('历史数据有空内容');
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
        Toast.error('请添加 Prompt 模板或输入提问内容');
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
          return Toast.error('历史数据有空内容');
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
            return Toast.error('历史数据有空内容');
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
                {title || '基准组'}
              </Typography.Text>
              {!isUndefined(uid) ? (
                <div className={styles['btn-group']}>
                  <Tooltip content="设置为基准组" theme="dark">
                    <IconButton
                      color="secondary"
                      size="small"
                      icon={<IconCozDubbleHorizontal />}
                      onClick={exchangePromptToDraft}
                      disabled={allStreaming}
                    />
                  </Tooltip>
                  <Tooltip content="删除对照组" theme="dark">
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
              value={modelConfig}
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
              models={service.data?.models}
              onModelChange={setCurrentModel}
              modelSelectProps={{
                className: 'w-full',
                loading: service.loading,
              }}
            />
          </div>
          <div className="flex-1 px-6 pr-[18px] py-3 flex flex-col gap-3  overflow-y-auto styled-scrollbar">
            <PromptEditorCard canCollapse uid={uid} defaultVisible />
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
            <Typography.Text strong>预览与调试</Typography.Text>
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
              停止响应
            </Button>
          ) : null}
        </div>
      </div>
    );
  },
);
