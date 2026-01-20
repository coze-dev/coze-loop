// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
import { useMemo } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { useModalData } from '@cozeloop/hooks';
import {
  EditIconButton,
  TextWithCopy,
  TooltipWhenDisabled,
} from '@cozeloop/components';
import {
  ContentType,
  Role,
  TemplateType,
  type Prompt,
  PromptType,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozLoading,
  IconCozBrace,
  IconCozPlus,
  IconCozMore,
  IconCozPlug,
  IconCozLongArrowTopRight,
  IconCozLongArrowUp,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Dropdown,
  IconButton,
  Popover,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import {
  getPlaceholderErrorContent,
  messageHasSnippetError,
  messagesHasSnippet,
  nextVersion,
} from '@/utils/prompt';
import {
  getButtonDisabledFromConfig,
  getButtonHiddenFromConfig,
} from '@/utils/base';
import { usePromptStore } from '@/store/use-prompt-store';
import {
  type CompareGroupLoop,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { usePrompt } from '@/hooks/use-prompt';
import { useCompare } from '@/hooks/use-compare';
import { EVENT_NAMES } from '@/consts';
import { SnippetUseageModal } from '@/components/snippet-useage-modal';
import { PromptSubmit } from '@/components/prompt-submit';
import { PromptDeleteModal } from '@/components/prompt-delete-modal';
import { PromptCreateModal } from '@/components/prompt-create-modal';

import { usePromptDevProviderContext } from '../prompt-provider';

export function PromptHeader() {
  const {
    spaceID,
    isPlayground,
    readonly: globalReadonly,
    buttonConfig,
    sendEvent,
    renderHeaderButtons,
    renderExtraHeaderDropdown,
    activeTab,
    extraTabs,
  } = usePromptDevProviderContext();
  const submitModal = useModalData();
  const deleteModal = useModalData<Prompt>();
  const snippetUseageModal = useModalData();

  const isNotDevTab = activeTab !== 'dev' && extraTabs?.length;

  const {
    autoSaving,
    versionChangeLoading,
    setVersionChangeVisible,
    versionChangeVisible,
    setVersionChangeLoading,
    readonly,
    setBasicReadonly,
  } = useBasicStore(
    useShallow(state => ({
      autoSaving: state.autoSaving,
      versionChangeLoading: state.versionChangeLoading,
      setVersionChangeVisible: state.setVersionChangeVisible,
      versionChangeVisible: state.versionChangeVisible,
      setVersionChangeLoading: state.setVersionChangeLoading,
      readonly: state.readonly,
      setBasicReadonly: state.setReadonly,
    })),
  );

  const {
    promptInfo,
    messageList,
    variables,
    modelConfig,
    currentModel,
    tools,
    toolCallConfig,
    templateType,
    setPromptInfo,
    totalReferenceCount,
  } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      messageList: state.messageList,
      variables: state.variables,
      modelConfig: state.modelConfig,
      currentModel: state.currentModel,
      tools: state.tools,
      toolCallConfig: state.toolCallConfig,
      templateType: state.templateType,
      setPromptInfo: state.setPromptInfo,
      totalReferenceCount: state.totalReferenceCount,
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

  const isSnippet =
    promptInfo?.prompt_basic?.prompt_type === PromptType.Snippet;

  // 更简洁的处理方式：直接判断类型并调用
  const delDisabled = getButtonDisabledFromConfig(
    buttonConfig?.deleteButton,
    promptInfo,
  );

  const onDeletePrompt = (item?: Prompt) => {
    if (!delDisabled && item?.prompt_key) {
      if (buttonConfig?.deleteButton?.onClick) {
        buttonConfig?.deleteButton?.onClick?.({ prompt: item });
      } else {
        deleteModal.open(item);
      }
    }
  };

  const { streaming } = useCompare();

  const { promptByVersionService } = usePrompt({
    promptID: promptInfo?.id,
    spaceID,
  });

  const promptInfoModal = useModalData<{
    prompt?: Prompt;
    isEdit?: boolean;
    isCopy?: boolean;
  }>();

  const handleSubmit = () => {
    if (autoSaving) {
      return;
    }
    if (buttonConfig?.submitButton?.onClick) {
      buttonConfig?.submitButton?.onClick?.({ prompt: promptInfo });
    } else {
      submitModal.open();
    }
  };

  const handleBackToDraft = () => {
    setVersionChangeLoading(true);
    promptByVersionService
      .runAsync({ version: '', withCommit: true })
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
  const snippetTypeError = messageHasSnippetError(
    messageList || [],
    templateType?.type || TemplateType.Normal,
  );

  const submitErrorTip = useMemo(() => {
    if (multiModalError) {
      return I18n.t('selected_model_not_support_multi_modal');
    }
    if (hasPlaceholderError) {
      return I18n.t('placeholder_var_error');
    }
    if (snippetTypeError) {
      return I18n.t('prompt_prompt_contains_mismatched_snippet');
    }
    if (!hasPeDraft) {
      return I18n.t('no_draft_change');
    }
    return '';
  }, [multiModalError, hasPlaceholderError, hasPeDraft, snippetTypeError]);

  const handleAddNewComparePrompt = () => {
    const newComparePrompt: CompareGroupLoop = {
      prompt_detail: {
        prompt_template: {
          template_type: templateType?.value as TemplateType,
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
    setVersionChangeVisible(false);
  };

  const compareBtn =
    isSnippet ||
    getButtonHiddenFromConfig(
      buttonConfig?.compareButton,
      promptInfo,
    ) ? null : (
      <Button
        color="primary"
        onClick={() => {
          handleAddNewComparePrompt();
          sendEvent?.(EVENT_NAMES.pe_mode_compare, {
            prompt_id: `${promptInfo?.id || 'playground'}`,
          });
        }}
        disabled={
          streaming ||
          versionChangeLoading ||
          readonly ||
          globalReadonly ||
          getButtonDisabledFromConfig(buttonConfig?.compareButton, promptInfo)
        }
        data-btm="d32140"
        data-btm-title={I18n.t('prompt_enter_compare_mode')}
      >
        {I18n.t('prompt_compare_mode')}
      </Button>
    );

  const versionListBtn =
    !isPlayground && !isNotDevTab ? (
      <Button
        color="primary"
        onClick={() => setVersionChangeVisible(v => Boolean(!v))}
        disabled={streaming}
        data-btm="d94594"
        data-btm-title={I18n.t('prompt_open_version_history')}
      >
        {I18n.t('version_record')}
      </Button>
    ) : null;

  const quickCreateBtn =
    !isPlayground ||
    getButtonHiddenFromConfig(buttonConfig?.createButton, promptInfo) ? null : (
      <TooltipWhenDisabled
        content={
          !modelConfig?.model_id
            ? I18n.t('prompt_please_select_a_model')
            : I18n.t('placeholder_var_create_error')
        }
        disabled={hasPlaceholderError || !modelConfig?.model_id}
        theme="dark"
      >
        <Button
          color="brand"
          onClick={() => {
            const newPromptInfo = {
              ...promptInfo,
              prompt_key: '',
              prompt_basic: {
                display_name: '',
                description: '',
              },
              prompt_commit: {
                detail: {
                  prompt_template: {
                    template_type: templateType?.value as TemplateType,
                    messages: messageList,
                    variable_defs: variables,
                    has_snippet: messagesHasSnippet(messageList || []),
                  },
                  tools,
                  tool_call_config: toolCallConfig,
                  model_config: modelConfig,
                },
              },
            };
            if (buttonConfig?.createButton?.onClick) {
              buttonConfig?.createButton?.onClick(newPromptInfo);
            } else {
              promptInfoModal.open({
                prompt: newPromptInfo,
              });
            }
          }}
          disabled={
            hasPlaceholderError ||
            streaming ||
            !modelConfig?.model_id ||
            getButtonDisabledFromConfig(buttonConfig?.createButton, promptInfo)
          }
          data-btm="d27573"
          data-btm-title={I18n.t('quick_create')}
        >
          {I18n.t('quick_create')}
        </Button>
      </TooltipWhenDisabled>
    );

  const submitBtn = useMemo(() => {
    if (
      isPlayground ||
      getButtonHiddenFromConfig(buttonConfig?.submitButton, promptInfo)
    ) {
      return null;
    }

    if (!versionChangeVisible && readonly && !globalReadonly) {
      return (
        <Button
          color="brand"
          onClick={handleBackToDraft}
          loading={versionChangeLoading}
          disabled={globalReadonly || streaming}
        >
          {I18n.t('revert_draft_version')}
        </Button>
      );
    }

    if (versionChangeVisible && readonly) {
      return null;
    }

    return (
      <TooltipWhenDisabled
        content={submitErrorTip}
        disabled={
          hasPlaceholderError ||
          !hasPeDraft ||
          multiModalError ||
          snippetTypeError
        }
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
            globalReadonly ||
            snippetTypeError ||
            getButtonDisabledFromConfig(buttonConfig?.submitButton, promptInfo)
          }
          data-btm="d64242"
          data-btm-title={I18n.t('prompt_submit_new_version')}
        >
          {I18n.t('prompt_submit_new_version')}
        </Button>
      </TooltipWhenDisabled>
    );
  }, [
    promptInfo,
    isPlayground,
    buttonConfig,
    versionChangeVisible,
    readonly,
    globalReadonly,
    streaming,
    versionChangeLoading,
    hasPlaceholderError,
    hasPeDraft,
    multiModalError,
    submitErrorTip,
    autoSaving,
  ]);

  return (
    <div
      className="flex justify-between items-center px-6 py-2 border-b !h-[56px] flex-shrink-0"
      data-btm="c83887"
    >
      {isPlayground ? (
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
                ? formatTimestampToString(
                    promptInfo?.prompt_draft?.draft_info?.updated_at,
                  )
                : ''}
            </Tag>
          )}
        </div>
      ) : (
        <div className="flex items-center gap-2">
          {getButtonHiddenFromConfig(
            buttonConfig?.backButton,
            promptInfo,
          ) ? null : (
            <IconButton
              color="secondary"
              className="!w-[32px] !h-[32px]"
              icon={
                <IconCozLongArrowUp
                  className="-rotate-90 text-[20px] cursor-pointer shrink-0 !coz-fg-plus !font-medium"
                  onClick={() => {
                    if (buttonConfig?.backButton?.onClick) {
                      buttonConfig?.backButton?.onClick({
                        prompt: promptInfo,
                      });
                    } else {
                      history.back();
                    }
                  }}
                />
              }
              disabled={getButtonDisabledFromConfig(
                buttonConfig?.backButton,
                promptInfo,
              )}
            />
          )}
          <div className="flex gap-3 items-center">
            <div className="flex items-center gap-1">
              <div className="flex items-center gap-2 cursor-pointer">
                <Popover
                  position="bottomLeft"
                  className="!p-0"
                  content={
                    <div className="w-[330px] px-4 py-6 flex flex-col gap-3 justify-center items-center">
                      <div
                        className="w-14 h-14 rounded-[8px] flex items-center justify-center text-white"
                        style={{ background: '#B0B9FF' }}
                        onClick={e => e.stopPropagation()}
                      >
                        <IconCozBrace fontSize={24} />
                      </div>
                      <Typography.Title
                        heading={6}
                        className="!max-w-[260px]"
                        ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
                        onClick={e => e.stopPropagation()}
                      >
                        {promptInfo?.prompt_basic?.display_name}
                      </Typography.Title>
                      {getButtonHiddenFromConfig(
                        buttonConfig?.viewCodeButton,
                      ) ? null : (
                        <Tag
                          color="primary"
                          className="!py-1 cursor-pointer"
                          prefixIcon={<IconCozPlug />}
                          onClick={() => {
                            sendEvent?.(EVENT_NAMES.prompt_click_view_code, {
                              prompt_id: `${promptInfo?.id || 'playground'}`,
                            });
                            if (
                              getButtonDisabledFromConfig(
                                buttonConfig?.viewCodeButton,
                                promptInfo,
                              )
                            ) {
                              return;
                            }
                            if (buttonConfig?.viewCodeButton?.onClick) {
                              buttonConfig?.viewCodeButton?.onClick({
                                prompt: promptInfo,
                              });
                            } else {
                              window.open(
                                'https://loop.coze.cn/open/docs/cozeloop/sdk',
                              );
                            }
                          }}
                        >
                          {I18n.t('prompt_use_sdk')}
                          <IconCozLongArrowTopRight />
                        </Tag>
                      )}
                      <TextWithCopy
                        content={promptInfo?.prompt_key}
                        maxWidth={260}
                        copyTooltipText={I18n.t('copy_prompt_key')}
                      />
                    </div>
                  }
                  clickToHide
                >
                  <div
                    className="w-8 h-8 rounded-[8px] flex items-center justify-center text-white"
                    style={{ background: '#B0B9FF' }}
                  >
                    <IconCozBrace />
                  </div>
                </Popover>
                <Typography.Text
                  className="!font-medium !max-w-[200px] !text-[14px] !leading-[20px] !coz-fg-plus"
                  ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
                >
                  {promptInfo?.prompt_basic?.display_name}
                </Typography.Text>
              </div>
              <EditIconButton
                data-btm="d35552"
                data-btm-title={I18n.t('edit_prompt')}
                onClick={() => {
                  if (buttonConfig?.editButton?.onClick) {
                    buttonConfig?.editButton?.onClick({ prompt: promptInfo });
                  } else {
                    promptInfoModal.open({
                      prompt: promptInfo,
                      isEdit: true,
                      isCopy: false,
                    });
                  }
                }}
              />
            </div>
            <div className="flex gap-2 items-center">
              {promptInfo?.prompt_draft || promptInfo?.prompt_commit ? (
                <Tag
                  color={isDraftEdit ? 'yellow' : 'brand'}
                  className="!py-0.5"
                >
                  {isDraftEdit
                    ? I18n.t('unsubmitted_changes')
                    : I18n.t('submitted')}
                </Tag>
              ) : null}
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
                    ? formatTimestampToString(
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
            {isSnippet ? (
              <Button
                color="primary"
                disabled={!totalReferenceCount}
                onClick={() => snippetUseageModal.open()}
              >
                {I18n.t('prompt_number_of_projects_referencing', {
                  placeholder1: totalReferenceCount ?? 0,
                })}
              </Button>
            ) : null}
            {renderHeaderButtons ? (
              renderHeaderButtons(
                isPlayground
                  ? [compareBtn, quickCreateBtn]
                  : [compareBtn, versionListBtn, submitBtn],
                promptInfo,
              )
            ) : (
              <>
                {compareBtn}
                {versionListBtn}
                {quickCreateBtn}
                {submitBtn}
              </>
            )}
            {!isPlayground && !isSnippet ? (
              <Dropdown
                trigger="click"
                position="bottomRight"
                showTick={false}
                zIndex={8}
                render={
                  <Dropdown.Menu>
                    {renderExtraHeaderDropdown?.(promptInfo)}
                    {getButtonHiddenFromConfig(
                      buttonConfig?.copyButton,
                      promptInfo,
                    ) ||
                    promptInfo?.prompt_draft ||
                    !promptInfo?.prompt_basic?.latest_version ? null : (
                      <Dropdown.Item
                        className="!px-2"
                        onClick={() => {
                          if (buttonConfig?.copyButton?.onClick) {
                            buttonConfig?.copyButton?.onClick({
                              prompt: promptInfo,
                            });
                          } else {
                            promptInfoModal.open({
                              prompt: promptInfo,
                              isEdit: false,
                              isCopy: true,
                            });
                          }
                        }}
                        disabled={
                          streaming ||
                          versionChangeLoading ||
                          globalReadonly ||
                          getButtonDisabledFromConfig(
                            buttonConfig?.copyButton,
                            promptInfo,
                          )
                        }
                      >
                        {I18n.t('copy')}
                      </Dropdown.Item>
                    )}
                    {getButtonHiddenFromConfig(
                      buttonConfig?.deleteButton,
                      promptInfo,
                    ) ? null : (
                      <TooltipWhenDisabled
                        content={I18n.t('prompt_no_delete_permission')}
                        disabled={delDisabled}
                        theme="dark"
                      >
                        <Dropdown.Item
                          className={classNames('!px-2', {
                            'opacity-50': streaming || delDisabled,
                          })}
                          onClick={() => onDeletePrompt(promptInfo)}
                          disabled={streaming || delDisabled || readonly}
                          data-btm="d75445"
                          data-btm-title={I18n.t('delete_prompt')}
                        >
                          <Typography.Text type="danger">
                            {I18n.t('delete')}
                          </Typography.Text>
                        </Dropdown.Item>
                      </TooltipWhenDisabled>
                    )}
                  </Dropdown.Menu>
                }
                clickToHide
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
              disabled={streaming}
              data-btm="d33400"
              data-btm-title={I18n.t('prompt_exit_compare_mode')}
            >
              {I18n.t('prompt_exit_compare_mode')}
            </Button>
            <Button
              color="primary"
              icon={<IconCozPlus />}
              disabled={(compareConfig?.groups || []).length >= 3 || streaming}
              onClick={handleAddNewComparePrompt}
              data-btm="d29798"
              data-btm-title={I18n.t('add_control_group')}
            >
              {I18n.t('add_control_group')}
            </Button>
          </>
        )}
      </div>
      <PromptCreateModal
        spaceID={spaceID}
        visible={promptInfoModal.visible}
        onCancel={promptInfoModal.close}
        data={promptInfoModal.data?.prompt}
        isCopy={promptInfoModal.data?.isCopy}
        isEdit={promptInfoModal.data?.isEdit}
        onOk={res => {
          if (promptInfoModal.data?.isCopy) {
            buttonConfig?.copyButton?.onSuccess?.({ prompt: res });
          } else if (promptInfoModal.data?.isEdit) {
            setPromptInfo(v => ({
              ...v,
              prompt_basic: res?.prompt_basic,
            }));
            buttonConfig?.editButton?.onSuccess?.({ prompt: res });
          } else {
            buttonConfig?.createButton?.onSuccess?.({ prompt: res });
          }
          promptInfoModal.close();
        }}
        isSnippet={promptInfo?.prompt_basic?.prompt_type === PromptType.Snippet}
      />

      <PromptDeleteModal
        data={deleteModal.data}
        visible={deleteModal.visible}
        onCacnel={deleteModal.close}
        onOk={() => {
          deleteModal.close();
          if (buttonConfig?.deleteButton?.onSuccess) {
            buttonConfig?.deleteButton?.onSuccess?.({ prompt: promptInfo });
          } else {
            history.back();
          }
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

      <SnippetUseageModal
        spaceID={spaceID}
        visible={snippetUseageModal.visible}
        snippet={promptInfo}
        totalReferenceCount={totalReferenceCount}
        onCancel={snippetUseageModal.close}
        onOk={() => {
          snippetUseageModal.close();
        }}
        onVersionItemClick={versionPrompt => {
          buttonConfig?.promptJumpButton?.onClick?.({ prompt: versionPrompt });
        }}
      />
    </div>
  );
}
