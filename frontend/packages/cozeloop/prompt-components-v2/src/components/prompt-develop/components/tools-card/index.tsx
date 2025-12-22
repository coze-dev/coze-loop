// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useEffect, useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { I18n } from '@cozeloop/i18n-adapter';
import { useModalData } from '@cozeloop/hooks';
import { CollapseCard } from '@cozeloop/components';
import { ToolChoiceType, ToolType } from '@cozeloop/api-schema/prompt';
import {
  IconCozPlus,
  IconCozWarningCircle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { isGeminiV2Model } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';
import { EVENT_NAMES } from '@/consts';

import { usePromptDevProviderContext } from '../prompt-provider';
import { type ToolWithMock } from '../../../tools-item/type';
import { ToolModal } from '../../../tools-item/tool-modal';
import { GoogleSearchItem } from '../../../tools-item/google-search-item';
import { ToolItem } from '../../../tools-item';

interface ToolsCardProps {
  uid?: number;
  defaultVisible?: boolean;
}

export function ToolsCard({ uid, defaultVisible }: ToolsCardProps) {
  const { sendEvent } = usePromptDevProviderContext();
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

  const isGeminiV2 = isGeminiV2Model(currentModel);

  const googleSearchOpen = Boolean(
    tools?.find(it => it.type === ToolType.GoogleSearch),
  );

  const { readonly: basicReadonly } = useBasicStore(
    useShallow(state => ({ readonly: state.readonly })),
  );

  const [visible, setVisible] = useState(defaultVisible);

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
    sendEvent?.(EVENT_NAMES.prompt_tool_delete, {
      prompt_id: `${promptInfo?.id || 'playground'}`,
      tool_name: name,
    });
  };

  const setGoogleSearchOpen = (open: boolean) => {
    setToolCallConfig({
      tool_choice: ToolChoiceType.Auto,
    });
    setTools(prev => {
      const oldTools =
        prev?.filter(it => it.type !== ToolType.GoogleSearch) || [];
      if (open) {
        return [
          {
            type: ToolType.GoogleSearch,
          },
          ...oldTools,
        ];
      }
      return oldTools;
    });
  };

  const toolSelectValue = useMemo(() => {
    if (toolCallConfig?.tool_choice === ToolChoiceType.Specific) {
      return toolCallConfig?.tool_choice_specification?.name || '';
    }
    return toolCallConfig?.tool_choice;
  }, [toolCallConfig]);

  const handleToolSelect = (tool: string) => {
    if (tool === ToolChoiceType.None) {
      setToolCallConfig({
        tool_choice: ToolChoiceType.None,
      });
      setUserDebugConfig({
        single_step_debug: false,
      });
    } else if (tool === ToolChoiceType.Auto) {
      setToolCallConfig({
        tool_choice: ToolChoiceType.Auto,
      });
      setUserDebugConfig({
        single_step_debug: true,
      });
    } else {
      setToolCallConfig({
        tool_choice: ToolChoiceType.Specific,
        tool_choice_specification: {
          type: ToolType.Function,
          name: tool,
        },
      });
      setUserDebugConfig({
        single_step_debug: true,
      });
    }
  };

  useEffect(() => {
    if (!isGeminiV2) {
      setTools(prev => {
        const oldTools = prev?.filter(it => it?.type !== ToolType.GoogleSearch);

        return oldTools;
      });
    }
  }, [isGeminiV2]);

  return (
    <>
      <CollapseCard
        subInfo={
          functionCallEnabled || !currentModel ? null : (
            <Tag size="mini" color="red" prefixIcon={<IconCozWarningCircle />}>
              {I18n.t('model_not_support')}
            </Tag>
          )
        }
        title={<Typography.Text strong>{I18n.t('function')}</Typography.Text>}
        extra={
          googleSearchOpen ? null : (
            <Space spacing="tight">
              {/* <div
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
            </div> */}
              <div onClick={e => e.stopPropagation()}>
                <Select
                  dropdownClassName="w-[120px]"
                  size="small"
                  onChange={v => handleToolSelect(v as string)}
                  value={toolSelectValue}
                  disabled={currentReadonly || !functionCallEnabled}
                >
                  <Select.Option value={ToolChoiceType.None}>
                    <Typography.Text>None</Typography.Text>
                  </Select.Option>
                  <Select.Option value={ToolChoiceType.Auto}>
                    <Typography.Text>Auto</Typography.Text>
                  </Select.Option>
                  {tools?.map(tool => (
                    <Select.Option
                      key={tool?.function?.name}
                      value={tool?.function?.name}
                    >
                      <Typography.Text
                        className="!max-w-[100px]"
                        ellipsis={{ showTooltip: true }}
                      >
                        {tool?.function?.name}
                      </Typography.Text>
                    </Select.Option>
                  ))}
                </Select>
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

                  <Typography.Text size="small">
                    {I18n.t('single_step_debugging')}
                  </Typography.Text>
                </div>
              )}
            </Space>
          )
        }
        visible={visible}
        onVisibleChange={setVisible}
      >
        <div className="flex flex-col gap-2 pt-4">
          {isGeminiV2 ? (
            <GoogleSearchItem
              value={googleSearchOpen}
              onChange={setGoogleSearchOpen}
              disabled={currentReadonly || !functionCallEnabled}
            />
          ) : null}
          {tools
            ?.filter(it => it.type !== ToolType.GoogleSearch)
            ?.map(item => {
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
            data-btm="d53629"
            data-btm-title={I18n.t('prompt_add_function')}
          >
            {I18n.t('prompt_add_function')}
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
            sendEvent?.(EVENT_NAMES.prompt_function_call_add, {
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
