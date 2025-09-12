// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import React, { useMemo } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import {
  getPlaceholderErrorContent,
  PromptCreate,
} from '@cozeloop/prompt-components';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import {
  EditIconButton,
  TextWithCopy,
  TooltipWhenDisabled,
} from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { useModalData } from '@cozeloop/base-hooks';
import { ContentType, Role, type Prompt } from '@cozeloop/api-schema/prompt';
import {
  IconCozLoading,
  IconCozBrace,
  IconCozPlus,
  IconCozLongArrowUp,
  IconCozMore,
  IconCozExit,
  IconCozBattle,
  IconCozPlug,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Dropdown,
  IconButton,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { convertDisplayTime, nextVersion } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import {
  type CompareGroupLoop,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { usePrompt } from '@/hooks/use-prompt';
import { useCompare } from '@/hooks/use-compare';

import { PromptSubmit } from '../prompt-submit';
import { PromptDelete } from '../prompt-delete';

export function PromptHeader() {
  const globalDisabled = useGuard({ point: GuardPoint['pe.prompt.global'] });
  const navigate = useNavigateModule();

  const submitModal = useModalData();
  const deleteModal = useModalData<Prompt>();

  const onDeletePrompt = (item?: Prompt) => {
    item?.prompt_key && deleteModal.open(item);
  };

  const {
    autoSaving,
    versionChangeLoading,
    setVersionChangeVisible,
    versionChangeVisible,
    setVersionChangeLoading,
    setExecuteHistoryVisible,
    readonly,
    setBasicReadonly,
  } = useBasicStore(
    useShallow(state => ({
      autoSaving: state.autoSaving,
      versionChangeLoading: state.versionChangeLoading,
      setVersionChangeVisible: state.setVersionChangeVisible,
      versionChangeVisible: state.versionChangeVisible,
      setVersionChangeLoading: state.setVersionChangeLoading,
      setExecuteHistoryVisible: state.setExecuteHistoryVisible,
      readonly: state.readonly,
      setBasicReadonly: state.setReadonly,
    })),
  );

  const {
    promptInfo,
    setPromptInfo,
    messageList,
    variables,
    modelConfig,
    currentModel,
    tools,
    toolCallConfig,
    templateType,
  } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      setPromptInfo: state.setPromptInfo,
      messageList: state.messageList,
      variables: state.variables,
      modelConfig: state.modelConfig,
      currentModel: state.currentModel,
      tools: state.tools,
      toolCallConfig: state.toolCallConfig,
      templateType: state.templateType,
    })),
  );

  const {
    setHistoricMessage,
    compareConfig,
    setCompareConfig,
    mockTools,
    mockVariables,
  } = usePromptMockDataStore(
    useShallow(state => ({
      setHistoricMessage: state.setHistoricMessage,
      setCompareConfig: state.setCompareConfig,
      compareConfig: state.compareConfig,
      mockVariables: state.mockVariables,
      mockTools: state.mockTools,
    })),
  );

  const { streaming } = useCompare();

  const { getPromptByVersion } = usePrompt({ promptID: promptInfo?.id });

  const promptInfoModal = useModalData<{
    prompt?: Prompt;
    isEdit?: boolean;
    isCopy?: boolean;
  }>();

  const handleSubmit = () => {
    if (autoSaving) {
      return;
    }
    submitModal.open();
  };

  const handleBackToDraft = () => {
    setVersionChangeLoading(true);
    getPromptByVersion('', true)
      .then(() => {
        setVersionChangeLoading(false);
        setBasicReadonly(false);
      })
      .catch(() => {
        setVersionChangeLoading(false);
        setBasicReadonly(false);
      });
  };

  const isDraftEdit = promptInfo?.prompt_draft?.draft_info?.is_modified;
  const hasPeDraft = Boolean(promptInfo?.prompt_draft);

  const hasPlaceholderError = useMemo(
    () =>
      messageList?.some(message => {
        if (message.role === Role.Placeholder) {
          return Boolean(getPlaceholderErrorContent(message, variables));
        }
        return false;
      }),
    [messageList, variables],
  );

  const isMultiModalModel = currentModel?.ability?.multi_modal;
  const multiModalError = messageList?.some(message => {
    if (
      message.parts?.some(
        part => part.type === ContentType.MultiPartVariable,
      ) &&
      !isMultiModalModel
    ) {
      return true;
    }
    return false;
  });

  const submitErrorTip = useMemo(() => {
    if (multiModalError) {
      return '所选模型不支持多模态，请调整变量类型或更换模型';
    }
    if (hasPlaceholderError) {
      return 'Placeholder 变量不存在或命名错误';
    }
    if (!hasPeDraft) {
      return '当前无草稿变更';
    }
    return '';
  }, [multiModalError, hasPlaceholderError, hasPeDraft]);

  const renderSubmitBtn = () => {
    if (!promptInfo?.prompt_key) {
      return null;
    }
    if (!versionChangeVisible && readonly && !globalDisabled.data.readonly) {
      return (
        <Button
          color="brand"
          onClick={handleBackToDraft}
          loading={versionChangeLoading}
          disabled={globalDisabled.data.readonly || streaming}
        >
          返回草稿版本
        </Button>
      );
    }

    if (versionChangeVisible) {
      return null;
    }

    return (
      <TooltipWhenDisabled
        content={submitErrorTip}
        disabled={hasPlaceholderError || !hasPeDraft || multiModalError}
        theme="dark"
      >
        <Button
          color="brand"
          onClick={handleSubmit}
          disabled={
            streaming ||
            hasPlaceholderError ||
            versionChangeLoading ||
            !hasPeDraft ||
            multiModalError ||
            globalDisabled.data.readonly
          }
        >
          提交新版本
        </Button>
      </TooltipWhenDisabled>
    );
  };

  const handleBack = () => {
    navigate('pe/prompts');
  };

  const handleAddNewComparePrompt = () => {
    const newComparePrompt: CompareGroupLoop = {
      prompt_detail: {
        prompt_template: {
          template_type: templateType,
          messages: messageList?.map(it => ({ ...it, key: nanoid() })),
          variable_defs: variables,
        },
        model_config: modelConfig,
        tools,
        tool_call_config: toolCallConfig,
      },
      debug_core: {
        mock_contexts: [],
        mock_variables: mockVariables,
        mock_tools: mockTools,
      },
      streaming: false,
      currentModel,
    };

    setCompareConfig(prev => {
      const newCompareConfig = {
        ...prev,
        groups: [
          ...(prev?.groups?.map(it => ({
            ...it,
            debug_core: { ...it.debug_core, mock_contexts: [] },
          })) || []),
          newComparePrompt,
        ],
      };
      return newCompareConfig;
    });
    setHistoricMessage([]);
  };

  return (
    <div className="flex justify-between items-center px-6 py-2 border-b !h-[62px]">
      {!promptInfo?.prompt_key ? (
        <div className="flex items-center gap-x-2">
          <h1 className="text-[20px] font-medium">Playground</h1>
          {autoSaving ? (
            <Tag
              color="primary"
              className="!py-0.5"
              prefixIcon={<IconCozLoading spin />}
            >
              草稿保存中...
            </Tag>
          ) : (
            <Tag color="primary">
              草稿已自动保存于
              {promptInfo?.prompt_draft?.draft_info?.updated_at
                ? convertDisplayTime(
                    promptInfo?.prompt_draft?.draft_info?.updated_at,
                  )
                : ''}
            </Tag>
          )}
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <IconButton
            className="flex-shrink-0"
            icon={
              <IconCozLongArrowUp className="w-5 h-5 rotate-[270deg] coz-fg-plus" />
            }
            color="secondary"
            onClick={handleBack}
          />
          <div
            className="w-9 h-9 rounded-[8px] flex items-center justify-center text-white"
            style={{ background: '#B0B9FF' }}
          >
            <IconCozBrace />
          </div>
          <div className="flex flex-col">
            <div className="flex items-center gap-1">
              <Typography.Text
                className="!font-medium !max-w-[200px] !text-[14px] !leading-[20px] !coz-fg-plus"
                ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
              >
                {promptInfo?.prompt_basic?.display_name}
              </Typography.Text>

              <EditIconButton
                onClick={() => {
                  promptInfoModal.open({
                    prompt: promptInfo,
                    isEdit: true,
                    isCopy: false,
                  });
                }}
              />
            </div>
            <div className="flex gap-2 items-center">
              <TextWithCopy
                content={promptInfo.prompt_key}
                maxWidth={200}
                copyTooltipText="复制 Prompt Key"
                textClassName="!text-xs"
                textType="tertiary"
              />
              <Divider
                layout="vertical"
                style={{ height: 12, margin: '0 8px' }}
              />
              <Tag
                color="primary"
                className="!py-0.5 cursor-pointer"
                prefixIcon={<IconCozPlug />}
                onClick={() => {
                  sendEvent(EVENT_NAMES.prompt_click_view_code, {
                    prompt_id: `${promptInfo?.id || 'playground'}`,
                  });
                  window.open('https://loop.coze.cn/open/docs/cozeloop/sdk');
                }}
              >
                使用 SDK
              </Tag>
              <Divider
                layout="vertical"
                style={{ height: 12, margin: '0 8px' }}
              />
              {promptInfo.prompt_draft || promptInfo.prompt_commit ? (
                <Tag
                  color={isDraftEdit ? 'yellow' : 'brand'}
                  className="!py-0.5"
                >
                  {isDraftEdit ? '修改未提交' : '已提交'}
                </Tag>
              ) : null}
              {autoSaving ? (
                <Tag
                  color="primary"
                  className="!py-0.5"
                  prefixIcon={<IconCozLoading spin />}
                >
                  草稿保存中...
                </Tag>
              ) : isDraftEdit ? (
                <Tag color="primary" className="!py-0.5">
                  草稿已自动保存于
                  {promptInfo?.prompt_draft?.draft_info?.updated_at ||
                  promptInfo?.prompt_commit?.commit_info?.committed_at
                    ? convertDisplayTime(
                        `${
                          promptInfo?.prompt_draft?.draft_info?.updated_at ||
                          promptInfo?.prompt_commit?.commit_info?.committed_at
                        }`,
                      )
                    : ''}
                </Tag>
              ) : promptInfo?.prompt_commit?.commit_info?.version ||
                promptInfo?.prompt_draft?.draft_info?.base_version ? (
                <Tag color="primary" className="!py-0.5">
                  {promptInfo?.prompt_commit?.commit_info?.version ||
                    promptInfo?.prompt_draft?.draft_info?.base_version}
                </Tag>
              ) : null}
            </div>
          </div>
        </div>
      )}
      <div className="flex items-center space-x-2">
        {!compareConfig?.groups?.length ? (
          <>
            <Button
              color="primary"
              onClick={() => {
                handleAddNewComparePrompt();
                sendEvent(EVENT_NAMES.pe_mode_compare, {
                  prompt_id: `${promptInfo?.id || 'playground'}`,
                });
              }}
              icon={<IconCozBattle />}
              disabled={
                streaming ||
                versionChangeLoading ||
                readonly ||
                globalDisabled.data.readonly
              }
            >
              进入自由对比模式
            </Button>
            {promptInfo?.prompt_key ? (
              <Button
                color="primary"
                onClick={() => setVersionChangeVisible(v => Boolean(!v))}
                disabled={streaming}
              >
                版本记录
              </Button>
            ) : null}
            {promptInfo?.prompt_key ? null : (
              <TooltipWhenDisabled
                content={
                  !modelConfig?.model_id
                    ? '请选择一个模型'
                    : 'Placeholder 变量名不存在或命名错误，无法创建'
                }
                disabled={hasPlaceholderError || !modelConfig?.model_id}
                theme="dark"
              >
                <Button
                  color="brand"
                  onClick={() => {
                    promptInfoModal.open({
                      prompt: {
                        ...promptInfo,
                        prompt_commit: {
                          detail: {
                            prompt_template: {
                              template_type: templateType,
                              messages: messageList,
                              variable_defs: variables,
                            },
                            tools,
                            tool_call_config: toolCallConfig,
                            model_config: modelConfig,
                          },
                        },
                      },
                    });
                  }}
                  disabled={
                    hasPlaceholderError || streaming || !modelConfig?.model_id
                  }
                >
                  快捷创建
                </Button>
              </TooltipWhenDisabled>
            )}
            {renderSubmitBtn()}
            {promptInfo?.prompt_key ? (
              <Dropdown
                trigger="click"
                position="bottomRight"
                showTick={false}
                zIndex={8}
                render={
                  <Dropdown.Menu>
                    <Dropdown.Item
                      className="!px-2"
                      onClick={() => setExecuteHistoryVisible(true)}
                    >
                      调试历史
                    </Dropdown.Item>
                    {readonly ? (
                      <Dropdown.Item
                        className="!px-2"
                        onClick={() =>
                          promptInfoModal.open({
                            prompt: promptInfo,
                            isEdit: false,
                            isCopy: true,
                          })
                        }
                        disabled={streaming || versionChangeLoading}
                      >
                        创建副本
                      </Dropdown.Item>
                    ) : null}
                    <Dropdown.Item
                      className="!px-2"
                      onClick={() => onDeletePrompt(promptInfo)}
                      disabled={streaming}
                    >
                      <Typography.Text type="danger">删除</Typography.Text>
                    </Dropdown.Item>
                  </Dropdown.Menu>
                }
              >
                <IconButton icon={<IconCozMore />} color="primary" />
              </Dropdown>
            ) : null}
          </>
        ) : (
          <>
            <Button
              color="primary"
              onClick={() => {
                setCompareConfig({ groups: [] });
                setHistoricMessage([]);
              }}
              icon={<IconCozExit />}
              disabled={streaming}
            >
              退出自由对比模式
            </Button>
            <Button
              color="primary"
              icon={<IconCozPlus />}
              disabled={(compareConfig?.groups || []).length >= 3 || streaming}
              onClick={handleAddNewComparePrompt}
            >
              增加对照组
            </Button>
          </>
        )}
      </div>
      <PromptCreate
        visible={promptInfoModal.visible}
        onCancel={promptInfoModal.close}
        data={promptInfoModal.data?.prompt}
        isCopy={promptInfoModal.data?.isCopy}
        isEdit={promptInfoModal.data?.isEdit}
        onOk={res => {
          if (promptInfoModal.data?.isCopy) {
            window.open(`pe/prompts/${res.cloned_prompt_id}`);
          } else if (promptInfoModal.data?.isEdit) {
            setPromptInfo(v => ({
              ...v,
              prompt_basic: res?.prompt_basic,
            }));
          } else {
            navigate(`pe/prompts/${res.id}`);
          }

          promptInfoModal.close();
        }}
      />
      <PromptSubmit
        visible={submitModal.visible}
        onCancel={submitModal.close}
        onOk={() => {
          submitModal.close();
          handleBackToDraft();
        }}
        initVersion={nextVersion(promptInfo?.prompt_basic?.latest_version)}
      />
      <PromptDelete
        data={deleteModal.data}
        visible={deleteModal.visible}
        onCacnel={deleteModal.close}
        onOk={() => {
          deleteModal.close();
          navigate('pe/prompts');
        }}
      />
    </div>
  );
}
