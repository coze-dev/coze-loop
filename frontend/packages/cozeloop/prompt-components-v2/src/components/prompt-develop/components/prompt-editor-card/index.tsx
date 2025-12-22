// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import Sortable from 'sortablejs';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import { useLatest } from 'ahooks';
import { safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { CollapseCard } from '@cozeloop/components';
import {
  Role,
  TemplateType,
  type VariableDef,
  VariableType,
} from '@cozeloop/api-schema/prompt';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import {
  getMockVariables,
  getMultiModalVariableKeys,
  getPlaceholderVariableKeys,
} from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';
import { type PromptMessage } from '@/components/prompt-editor/type';
import { PromptEditor } from '@/components/prompt-editor';

import { usePromptDevProviderContext } from '../prompt-provider';
import { EditorCardHeaderActions } from './editor-card-header-actions';
import { DiffMessageEditor } from './diff-message-editor';

interface PromptEditorCardProps {
  uid?: number;
  canCollapse?: boolean;
  configAreaVisible?: boolean;
  configExecuteVisible?: boolean;
  setConfigAreaVisible?: (visible: boolean) => void;
  setConfigExecuteVisible?: (visible: boolean) => void;
}

export function PromptEditorCard({
  canCollapse,
  uid,
  configAreaVisible = true,
  configExecuteVisible = true,
  setConfigAreaVisible,
  setConfigExecuteVisible,
}: PromptEditorCardProps) {
  const { renderEditorLeftActions, renderEditorRightActions, hideSnippet } =
    usePromptDevProviderContext();
  const sortableContainer = useRef<HTMLDivElement>(null);
  const {
    streaming,
    messageList = [],
    setMessageList,
    variables = [],
    mockVariables,
    setVariables,
    setMockVariables,
    currentModel,
  } = useCompare(uid);

  const variablesRef = useLatest(variables);

  const { readonly: basicReadonly } = useBasicStore(
    useShallow(state => ({
      readonly: state.readonly,
    })),
  );

  const { promptInfo, templateType } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      templateType: state.templateType,
    })),
  );

  const isNotNormalTemplate = templateType?.type !== TemplateType.Normal;

  const librarys = safeJsonParse<unknown[]>(
    (promptInfo?.prompt_commit || promptInfo?.prompt_draft)?.detail?.ext_infos
      ?.workflow ?? '[]',
  );

  const [isDrag, setIsDrag] = useState(false);
  const [inDiffEditor, setInDiffEditor] = useState(false);
  const readonly = basicReadonly || streaming;

  const handleAddMessage = () => {
    let messageType = Role.User;
    setMessageList(prev => {
      if (!prev?.length) {
        messageType = Role.System;
      } else if (prev?.[prev.length - 1]?.role === Role.User) {
        messageType = Role.Assistant;
      }
      const newInfo = (prev || [])?.concat({
        key: nanoid(),
        role: messageType,
        content: '',
      });
      return newInfo;
    });
  };

  const handleMessageTypeChange = (key?: string, role?: Role) => {
    setMessageList(prev => {
      const newInfo = prev?.map(it => {
        if (it.key === key) {
          if (it.role === Role.Placeholder || role === Role.Placeholder) {
            return {
              ...it,
              role,
              content: '',
              key: nanoid(),
            };
          }
          return { ...it, role };
        }
        return it;
      });
      return newInfo as any;
    });
  };

  const handleMessageChange = (key?: string, message?: PromptMessage) => {
    setMessageList(prev => {
      const newInfo = prev?.map(it => {
        if (it.key === key) {
          const { parts } = message || {};
          return {
            ...it,
            ...message,
            content: parts?.length ? '' : message?.content,
          };
        }
        return it;
      });

      return newInfo as any;
    });
  };

  const handleDeleteMessage = (key?: string) => {
    setMessageList(prev => {
      const newInfo = prev?.filter(it => it.key !== key);
      return newInfo;
    });
  };

  const insertSnippetVariables = useCallback(
    (newVariables?: VariableDef[]) => {
      if (newVariables?.length) {
        const array = (newVariables?.filter(
          it => !variables?.find(it1 => it1.key === it.key),
        ) || []) as VariableDef[];
        const newArray = [...(variables || []), ...array];
        setVariables(newArray);
        const newMockVariables = getMockVariables(
          newArray,
          mockVariables || [],
        );
        setMockVariables(newMockVariables);
      }
    },
    [variables, mockVariables],
  );

  useEffect(() => {
    if (isNotNormalTemplate && messageList?.length) {
      const normalVariables = variablesRef.current?.filter(
        it =>
          it.type !== VariableType.Placeholder &&
          it.type !== VariableType.MultiPart,
      );
      const normalVariableKeys = normalVariables?.map(it => it.key || '');

      const multiModalVariableArray = getMultiModalVariableKeys(
        messageList,
        normalVariableKeys,
      );

      if (multiModalVariableArray?.length) {
        normalVariableKeys.push(
          ...multiModalVariableArray.map(it => it.key || ''),
        );
        normalVariables.push(...multiModalVariableArray);
      }
      const placeholderVariableArray = getPlaceholderVariableKeys(
        messageList,
        normalVariableKeys,
      );
      if (placeholderVariableArray?.length) {
        normalVariableKeys.push(
          ...placeholderVariableArray.map(it => it.key || ''),
        );
        normalVariables.push(...placeholderVariableArray);
      }

      setVariables(normalVariables);
      const newMockVariables = getMockVariables(
        normalVariables,
        mockVariables || [],
      );
      setMockVariables(newMockVariables);
    }
  }, [isNotNormalTemplate, JSON.stringify(messageList)]);

  useEffect(() => {
    setMessageList([...(messageList || [])]);
  }, [templateType?.type]);

  useEffect(() => {
    if (sortableContainer.current) {
      new Sortable(sortableContainer.current, {
        animation: 150,
        handle: '.drag',
        onSort: evt => {
          setMessageList(list => {
            const draft = [...(list ?? [])];
            if (draft.length) {
              const { oldIndex = 0, newIndex = 0 } = evt;
              const [item] = draft.splice(oldIndex, 1);
              draft.splice(newIndex, 0, item);
            }
            return draft;
          });
        },
        onStart: () => setIsDrag(true),
        onEnd: () => setIsDrag(false),
      });
    }
  }, []);

  return (
    <CollapseCard
      data-btm-title="Prompt Template"
      data-btm="c82117"
      title={
        <div
          className={classNames(
            'flex items-center justify-between px-6 flex-shrink-0 !h-[42px]',
            {
              '!px-0 !h-auto': canCollapse,
            },
          )}
        >
          <div className="flex items-end gap-2">
            {canCollapse ? (
              <Typography.Text strong>
                {I18n.t('prompt_template')}
              </Typography.Text>
            ) : (
              <Typography.Title heading={6}>
                {I18n.t('prompt_template')}
              </Typography.Title>
            )}
            {/* <Typography.Text size="small" type="secondary">
             515 tokens
            </Typography.Text> */}
          </div>
          <EditorCardHeaderActions
            disabled={readonly}
            configAreaVisible={configAreaVisible}
            configExecuteVisible={configExecuteVisible}
            setConfigAreaVisible={setConfigAreaVisible}
            setConfigExecuteVisible={setConfigExecuteVisible}
            inDiffEditor={inDiffEditor}
            setInDiffEditor={setInDiffEditor}
          />
        </div>
      }
      disableCollapse={!canCollapse}
      className="overflow-hidden w-full h-full !gap-1"
    >
      <div
        className={classNames(
          'flex flex-col gap-2 pl-6 pr-[18px] styled-scrollbar',
          {
            'pt-4 !px-0': canCollapse,
          },
        )}
      >
        {inDiffEditor ? (
          <DiffMessageEditor
            prevVersion={
              promptInfo?.prompt_draft?.draft_info?.base_version ||
              promptInfo?.prompt_commit?.commit_info?.base_version
            }
            messageList={messageList}
            onMessageChange={handleMessageChange}
            onMessageTypeChange={handleMessageTypeChange}
            onDeleteMessage={handleDeleteMessage}
            currentModel={currentModel}
          />
        ) : (
          <div className="flex flex-col gap-2" ref={sortableContainer}>
            {messageList
              ?.filter(it => Boolean(it))
              ?.map(message => (
                <PromptEditor
                  key={`${message.key}-${templateType?.value}`}
                  message={message}
                  variables={variables?.filter(
                    it =>
                      it.type !== VariableType.Placeholder &&
                      it.type !== VariableType.MultiPart,
                  )}
                  disabled={readonly}
                  isInDrag={isDrag}
                  onMessageTypeChange={v =>
                    handleMessageTypeChange(message.key, v)
                  }
                  onMessageChange={v => handleMessageChange(message.key, v)}
                  minHeight={26}
                  onDelete={delMsg => handleDeleteMessage(delMsg?.key)}
                  modalVariableEnable={currentModel?.ability?.multi_modal}
                  modalVariableBtnHidden={message.role === Role.System}
                  cozeLibrarys={librarys}
                  leftActionBtns={renderEditorLeftActions?.({
                    message,
                    prompt: promptInfo,
                    messageList,
                  })}
                  rightActionBtns={renderEditorRightActions?.({
                    message,
                    prompt: promptInfo,
                    messageList,
                  })}
                  isJinja2Template={templateType?.type === TemplateType.Jinja2}
                  isGoTemplate={templateType?.type === TemplateType.GoTemplate}
                  insertSnippetVariables={insertSnippetVariables}
                  snippetBtnHidden={hideSnippet}
                ></PromptEditor>
              ))}
          </div>
        )}

        <Button
          color="primary"
          icon={<IconCozPlus />}
          onClick={handleAddMessage}
          disabled={readonly}
          data-btm="d79789"
          data-btm-title={I18n.t('add_message')}
        >
          {I18n.t('add_message')}
        </Button>
      </div>
    </CollapseCard>
  );
}
