/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useShallow } from 'zustand/react/shallow';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { CollapseCard } from '@cozeloop/components';
import { useModalData } from '@cozeloop/base-hooks';
import { ToolChoiceType } from '@cozeloop/api-schema/prompt';
import {
  IconCozPlus,
  IconCozWarningCircle,
} from '@coze-arch/coze-design/icons';
import { Button, Space, Switch, Tag, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';

import { ToolModal } from '../tools-item/tool-modal';
import { ToolItem, type ToolWithMock } from '../tools-item';

interface ToolsCardProps {
  uid?: number;
  defaultVisible?: boolean;
}

export function ToolsCard({ uid, defaultVisible }: ToolsCardProps) {
  const { promptInfo } = usePromptStore(
    useShallow(state => ({ promptInfo: state.promptInfo })),
  );

  const { userDebugConfig, setUserDebugConfig, compareConfig } =
    usePromptMockDataStore(
      useShallow(state => ({
        userDebugConfig: state.userDebugConfig,
        setUserDebugConfig: state.setUserDebugConfig,
        compareConfig: state.compareConfig,
      })),
    );

  const {
    streaming,
    currentModel,
    tools,
    setTools,
    toolCallConfig,
    setToolCallConfig,
    mockTools,
    setMockTools,
  } = useCompare(uid);

  const { readonly: basicReadonly } = useBasicStore(
    useShallow(state => ({ readonly: state.readonly })),
  );

  const isCompare = compareConfig?.groups?.length;

  const openTool = toolCallConfig?.tool_choice === ToolChoiceType.Auto;

  const toolModal = useModalData<ToolWithMock>();

  const currentReadonly = basicReadonly || streaming;
  const functionCallEnabled = currentModel?.ability?.function_call;

  const deleteToolByTool = (name?: string) => {
    if (!name) {
      return;
    }
    const toolList = (tools || []).filter(it => it.function?.name !== name);
    const mockToolList = (mockTools || []).filter(it => it?.name !== name);

    setTools([...toolList]);
    setMockTools([...mockToolList]);
    sendEvent(EVENT_NAMES.prompt_tool_delete, {
      prompt_id: `${promptInfo?.id || 'playground'}`,
      tool_name: name,
    });
  };

  return (
    <>
      <CollapseCard
        subInfo={
          functionCallEnabled || !currentModel ? null : (
            <Tag size="mini" color="red" prefixIcon={<IconCozWarningCircle />}>
              模型不支持
            </Tag>
          )
        }
        title={<Typography.Text strong>函数</Typography.Text>}
        extra={
          <Space spacing="tight">
            <div
              className="flex gap-1 items-center"
              onClick={e => e.stopPropagation()}
            >
              <Switch
                size="mini"
                checked={openTool}
                onChange={check => {
                  setToolCallConfig({
                    tool_choice: check
                      ? ToolChoiceType.Auto
                      : ToolChoiceType.None,
                  });
                  check &&
                    setUserDebugConfig({
                      single_step_debug: check,
                    });
                }}
                disabled={currentReadonly || !functionCallEnabled}
              />
              <Typography.Text size="small">启用函数</Typography.Text>
            </div>
            {isCompare ? null : (
              <div
                className="flex gap-1 items-center"
                onClick={e => e.stopPropagation()}
              >
                <Switch
                  size="mini"
                  checked={userDebugConfig?.single_step_debug}
                  onChange={check => {
                    setUserDebugConfig({
                      single_step_debug: check,
                    });
                  }}
                  disabled={
                    streaming ||
                    !functionCallEnabled ||
                    !openTool ||
                    currentReadonly
                  }
                />
                <Typography.Text size="small">单步调试</Typography.Text>
              </div>
            )}
          </Space>
        }
        defaultVisible={defaultVisible}
      >
        <div className="flex flex-col gap-2 pt-2">
          {tools?.map(item => {
            const mockTool = mockTools?.find(
              it => it?.name === item?.function?.name,
            );
            return (
              <ToolItem
                data={{ ...item, mock_response: mockTool?.mock_response }}
                onDelete={deleteToolByTool}
                onClick={() =>
                  toolModal.open({
                    ...item,
                    mock_response: mockTool?.mock_response,
                  })
                }
                showDelete={!currentReadonly}
              />
            );
          })}
          <Button
            color="primary"
            icon={<IconCozPlus />}
            onClick={() => toolModal.open()}
            disabled={currentReadonly || !functionCallEnabled}
          >
            新增函数
          </Button>
        </div>
      </CollapseCard>
      <ToolModal
        disabled={!functionCallEnabled || currentReadonly}
        visible={toolModal.visible}
        data={toolModal.data}
        onClose={() => toolModal.close()}
        onConfirm={(tool, isUpdate, oldData) => {
          const { mock_response, ...rest } = tool;
          const toolName = tool?.function?.name || '';
          const oldToolName = oldData?.function?.name || '';
          if (!isUpdate) {
            setTools?.(prev => [...(prev || []), rest]);
            setMockTools?.(prev => {
              const newMock = (prev || []).filter(it => it?.name !== toolName);
              return [
                ...newMock,
                { name: toolName, mock_response: tool?.mock_response },
              ];
            });
            sendEvent('prompt_function_call_add', {
              prompt_key: promptInfo?.prompt_key || 'playground',
              function_name: toolName,
            });
          } else {
            setTools?.(prev => {
              const newTools = (prev || []).map(it => {
                if (it.function?.name === oldToolName) {
                  return rest;
                }
                return it;
              });
              return newTools;
            });
            setMockTools?.(prev => {
              const newMock = (prev || []).filter(
                it => it?.name !== oldToolName,
              );
              return [
                ...newMock,
                { name: toolName, mock_response: tool?.mock_response },
              ];
            });
          }
          if (toolCallConfig?.tool_choice !== ToolChoiceType.Auto) {
            setToolCallConfig(prev => ({
              ...prev,
              tool_choice: ToolChoiceType.Auto,
            }));
          }
          setUserDebugConfig?.({ ...userDebugConfig, single_step_debug: true });
          toolModal.close();
        }}
        tools={tools}
      />
    </>
  );
}
