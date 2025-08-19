// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */

import { useNavigate } from 'react-router-dom';
import React, { useMemo } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import {
  getPlaceholderErrorContent,
  PromptCreate,
} from '@cozeloop/prompt-components';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EditIconButton,
  getBaseUrl,
  TextWithCopy,
  TooltipWhenDisabled,
} from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { useModalData } from '@cozeloop/base-hooks';
import { Role, TemplateType, type Prompt } from '@cozeloop/api-schema/prompt';
import {
  IconCozLoading,
  IconCozBrace,
  IconCozPlus,
  IconCozLongArrowUp,
  IconCozMore,
  IconCozExit,
  IconCozBattle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Dropdown,
  IconButton,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { convertDisplayTime } from '@/utils/prompt';
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
  const { spaceID } = useSpace();
  const baseURL = getBaseUrl(spaceID);
  const navigate = useNavigate();

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
  } = useBasicStore(
    useShallow(state => ({
      autoSaving: state.autoSaving,
      versionChangeLoading: state.versionChangeLoading,
      setVersionChangeVisible: state.setVersionChangeVisible,
      versionChangeVisible: state.versionChangeVisible,
      setVersionChangeLoading: state.setVersionChangeLoading,
      setExecuteHistoryVisible: state.setExecuteHistoryVisible,
      readonly: state.readonly,
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
      })
      .catch(() => {
        setVersionChangeLoading(false);
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

  const renderSubmitBtn = () => {
    if (!promptInfo?.prompt_key) {
      return null;
    }
    if (!versionChangeVisible && readonly) {
      return (
        <Button
          color="brand"
          onClick={handleBackToDraft}
          loading={versionChangeLoading}
          disabled={streaming}
        >
          {I18n.t('revert_draft_version')}
        </Button>
      );
    }

    if (versionChangeVisible) {
      return null;
    }

    return (
      <TooltipWhenDisabled
        content={
          !hasPeDraft
            ? I18n.t('no_draft_change')
            : I18n.t('placeholder_var_create_error')
        }
        disabled={hasPlaceholderError || !hasPeDraft}
        theme="dark"
      >
        <Button
          color="brand"
          onClick={handleSubmit}
          disabled={
            streaming ||
            hasPlaceholderError ||
            versionChangeLoading ||
            !hasPeDraft
          }
        >
          {I18n.t('submit_new_version')}
        </Button>
      </TooltipWhenDisabled>
    );
  };

  const handleBack = () => {
    navigate(`${getBaseUrl(spaceID)}/pe/prompts`);
  };

  const handleAddNewComparePrompt = () => {
    const newComparePrompt: CompareGroupLoop = {
      prompt_detail: {
        prompt_template: {
          template_type: TemplateType.Normal,
          messages: messageList,
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
              {I18n.t('draft_saving')}
            </Tag>
          ) : (
            <Tag color="primary">
              {I18n.t('draft_auto_saved_in')}
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
                copyTooltipText={I18n.t('copy_prompt_key')}
                textClassName="!text-xs"
                textType="tertiary"
              />
              <Divider
                layout="vertical"
                style={{ height: 12, margin: '0 8px' }}
              />
              {promptInfo.prompt_draft || promptInfo.prompt_commit ? (
                <Tag
                  color={isDraftEdit ? 'yellow' : 'brand'}
                  className="!py-0.5"
                >
                  {isDraftEdit
                    ? I18n.t('changes_not_submitted')
                    : I18n.t('submitted')}
                </Tag>
              ) : (
                <Tag
                  color={isDraftEdit ? 'yellow' : 'brand'}
                  className="!py-0.5"
                >
                  {I18n.t('changes_not_submitted')}
                </Tag>
              )}
              {autoSaving ? (
                <Tag
                  color="primary"
                  className="!py-0.5"
                  prefixIcon={<IconCozLoading spin />}
                >
                  {I18n.t('draft_saving')}
                </Tag>
              ) : isDraftEdit ? (
                <Tag color="primary" className="!py-0.5">
                  {I18n.t('draft_auto_saved_in')}
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
              disabled={streaming || versionChangeLoading || readonly}
            >
              {I18n.t('enter_free_comparison_mode')}
            </Button>
            {promptInfo?.prompt_key ? (
              <Button
                color="primary"
                onClick={() => setVersionChangeVisible(v => Boolean(!v))}
                disabled={streaming}
              >
                {I18n.t('version_record')}
              </Button>
            ) : null}
            {promptInfo?.prompt_key ? null : (
              <TooltipWhenDisabled
                content={I18n.t('placeholder_var_create_error')}
                disabled={hasPlaceholderError}
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
                              template_type: TemplateType.Normal,
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
                  disabled={hasPlaceholderError || streaming}
                >
                  {I18n.t('quick_create')}
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
                      {I18n.t('debug_history')}
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
                        {I18n.t('create_copy')}
                      </Dropdown.Item>
                    ) : null}
                    <Dropdown.Item
                      className="!px-2"
                      onClick={() => onDeletePrompt(promptInfo)}
                      disabled={streaming}
                    >
                      <Typography.Text type="danger">
                        {I18n.t('delete')}
                      </Typography.Text>
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
              {I18n.t('exit_free_comparison_mode')}
            </Button>
            <Button
              color="primary"
              icon={<IconCozPlus />}
              disabled={(compareConfig?.groups || []).length >= 3 || streaming}
              onClick={handleAddNewComparePrompt}
            >
              {I18n.t('add_control_group')}
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
            window.open(`${baseURL}/pe/prompts/${res.cloned_prompt_id}`);
          } else if (promptInfoModal.data?.isEdit) {
            setPromptInfo(v => ({
              ...v,
              prompt_basic: res?.prompt_basic,
            }));
          } else {
            navigate(`${baseURL}/pe/prompts/${res.id}`);
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
      />
      <PromptDelete
        data={deleteModal.data}
        visible={deleteModal.visible}
        onCacnel={deleteModal.close}
        onOk={() => {
          deleteModal.close();
          navigate(`${baseURL}/pe/prompts`);
        }}
      />
    </div>
  );
}
