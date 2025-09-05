/* eslint-disable security/detect-object-injection */
/* eslint-disable complexity */
import { type CSSProperties, useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import cn from 'classnames';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { type MockTool, type DebugToolCall } from '@cozeloop/api-schema/prompt';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Input, Tag, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';

import styles from './index.module.less';

interface FunctionItemProps {
  style?: CSSProperties;
  item: DebugToolCall;
  active?: boolean;
  isNotInTools?: boolean;
  stepDebugger?: boolean;
  onOutputChange?: (text?: string) => void;
}
export function FunctionItem({
  style,
  item,
  active,
  isNotInTools,
  onOutputChange,
  stepDebugger,
}: FunctionItemProps) {
  return (
    <div
      className={cn(styles['function-item'], {
        [styles['function-active']]: active,
        [styles['function-error']]: isNotInTools,
      })}
      style={style}
    >
      <Typography.Text
        className="variable-text"
        ellipsis={{ showTooltip: true }}
      >
        {item?.tool_call?.function_call?.name}
      </Typography.Text>
      <div className="flex gap-1 overflow-hidden w-full items-center">
        <Typography.Text size="small" type="tertiary" className="shrink-0">
          输入
        </Typography.Text>
        <Typography.Text
          ellipsis={{
            showTooltip: {
              opts: {
                position: 'top',
                content: (
                  <div className="max-h-[450px] overflow-y-auto overflow-x-hidden">
                    {item?.tool_call?.function_call?.arguments || '-'}
                  </div>
                ),
              },
            },
          }}
          size="small"
          type="tertiary"
          className="w-full"
        >
          {item?.tool_call?.function_call?.arguments || '-'}
        </Typography.Text>
      </div>
      <div className="flex gap-1 overflow-hidden w-full items-center">
        <Typography.Text size="small" type="tertiary" className="shrink-0">
          输出
        </Typography.Text>
        {!(active && stepDebugger) ? (
          <Typography.Text
            ellipsis={{
              showTooltip: {
                opts: {
                  position: 'top',
                  content: (
                    <div className="max-h-[450px] overflow-y-auto overflow-x-hidden">
                      {item?.mock_response || '-'}
                    </div>
                  ),
                },
              },
            }}
            size="small"
            type="tertiary"
            className="w-full"
          >
            {item?.mock_response || '-'}
          </Typography.Text>
        ) : (
          <Input
            onChange={onOutputChange}
            defaultValue={item.mock_response}
            placeholder="请输入模拟值"
            autoFocus
            className="w-full"
            borderless
            style={{ border: 0, borderRadius: 0, height: 22 }}
            size="small"
          />
        )}
      </div>
    </div>
  );
}

interface FunctionListProps {
  streaming?: boolean;
  toolCalls: Array<DebugToolCall>;
  hasMessage?: boolean;
  stepDebuggingTrace?: string;
  tools?: MockTool[];
  setToolCalls?: React.Dispatch<React.SetStateAction<DebugToolCall[]>>;
}

export function FunctionList({
  toolCalls,
  streaming,
  stepDebuggingTrace,
  tools,
  setToolCalls,
}: FunctionListProps) {
  const [isExpand, setIsExpand] = useState(true);

  const { promptInfo } = usePromptStore(
    useShallow(state => ({ promptInfo: state.promptInfo })),
  );
  const { userDebugConfig } = usePromptMockDataStore(
    useShallow(state => ({
      userDebugConfig: state.userDebugConfig,
    })),
  );
  const stepDebugger = userDebugConfig?.single_step_debug;

  const onOutputChange = (text: string, id?: string) => {
    const arr = toolCalls.map(item => {
      if (item?.tool_call?.id === id) {
        return { ...item, mock_response: text };
      }
      return item;
    });

    setToolCalls?.(arr);

    sendEvent(EVENT_NAMES.prompt_step_debugger_mock_change, {
      prompt_id: `${promptInfo?.id || 'playground'}`,
      change: true,
      upload_type: 'mock',
    });
  };

  const groupToolByTraceKey = useMemo(
    () =>
      toolCalls.reduce<Record<string, DebugToolCall[]>>((acc, tool) => {
        //TODO: 临时处理，后续移除 tool?.tool_call?.id
        const key = tool.debug_trace_key || tool?.tool_call?.id || '';
        acc[key] = acc[key] || [];
        acc[key].push(tool);
        return acc;
      }, {}),
    [toolCalls],
  );

  const keysLength = Object.keys(groupToolByTraceKey).length;

  return (
    <div className="flex flex-col gap-2">
      <Tag
        className="cursor-pointer"
        color="primary"
        onClick={() => setIsExpand(v => !v)}
        style={{ maxWidth: 'fit-content' }}
        suffixIcon={
          <IconCozArrowDown
            className={cn(styles['function-chevron-icon'], {
              [styles['function-chevron-icon-close']]: !isExpand,
            })}
            fontSize={12}
          />
        }
      >
        函数调用
      </Tag>
      {isExpand ? (
        <div className="flex flex-col gap-2">
          {Object.keys(groupToolByTraceKey).map((key, index) => (
            <div key={key} className="flex flex-col gap-2">
              {keysLength > 1 && (
                <div className={styles['function-order']}>{index + 1}</div>
              )}
              <div className="flex flex-wrap gap-2">
                {groupToolByTraceKey[key].map(tool => {
                  const isActive =
                    stepDebuggingTrace === tool.debug_trace_key && streaming;
                  const hasItem = tools?.some(
                    it => it?.name === tool.tool_call?.function_call?.name,
                  );
                  return (
                    <FunctionItem
                      key={tool.tool_call?.id}
                      item={tool}
                      active={isActive}
                      onOutputChange={text =>
                        onOutputChange(text || '', tool.tool_call?.id)
                      }
                      isNotInTools={!hasItem}
                      stepDebugger={stepDebugger}
                      style={{
                        maxWidth:
                          groupToolByTraceKey[key].length > 1
                            ? 'calc(50% - 6px)'
                            : '100%',
                      }}
                    />
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
