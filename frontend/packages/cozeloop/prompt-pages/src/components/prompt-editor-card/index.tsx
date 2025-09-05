/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import Sortable from 'sortablejs';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import { useLatest } from 'ahooks';
import { safeJsonParse } from '@cozeloop/toolkit';
import { CollapseCard } from '@cozeloop/components';
import { Role, TemplateType, VariableType } from '@cozeloop/api-schema/prompt';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import {
  getMockVariables,
  getMultiModalVariableKeys,
  getPlaceholderVariableKeys,
} from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';

import LibraryBlockWidget from '../loop-prompt-editor/widgets/skill';
import { LoopPromptEditor } from '../loop-prompt-editor';
import { TemplateSelect } from './template-select';

interface PromptEditorCardProps {
  uid?: number;
  canCollapse?: boolean;
  defaultVisible?: boolean;
}

export function PromptEditorCard({
  canCollapse,
  defaultVisible,
  uid,
}: PromptEditorCardProps) {
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

  const isNotNormalTemplate = templateType !== TemplateType.Normal;

  const { compareConfig } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
    })),
  );

  const librarys = safeJsonParse(
    (promptInfo?.prompt_commit || promptInfo?.prompt_draft)?.detail?.ext_infos
      ?.workflow ?? '[]',
  );

  const [isDrag, setIsDrag] = useState(false);
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

  const firstSpIndex = messageList?.findIndex(it => it.role === Role.System);

  return (
    <CollapseCard
      title={
        <div className="flex items-center justify-between">
          <Typography.Text strong>Prompt 模板</Typography.Text>
          {compareConfig?.groups?.length ? null : (
            <TemplateSelect streaming={streaming || readonly} />
          )}
        </div>
      }
      defaultVisible={defaultVisible}
      disableCollapse={!canCollapse}
    >
      <div
        className={classNames('flex flex-col gap-2', {
          'pt-3': canCollapse,
        })}
      >
        <div className={'flex flex-col gap-2'} ref={sortableContainer}>
          {messageList
            ?.filter(it => Boolean(it))
            ?.map((message, idx) => {
              const isFirstSp =
                message.role === Role.System && idx === firstSpIndex;
              return (
                <LoopPromptEditor
                  key={`${message.key}-${templateType}`}
                  message={message}
                  variables={variables?.filter(
                    it =>
                      it.type !== VariableType.Placeholder &&
                      it.type !== VariableType.MultiPart,
                  )}
                  disabled={readonly}
                  isDrag={isDrag}
                  onMessageTypeChange={v => {
                    setMessageList(prev => {
                      const newInfo = prev?.map(it => {
                        if (it.key === message.key) {
                          if (
                            it.role === Role.Placeholder ||
                            v === Role.Placeholder
                          ) {
                            return {
                              ...it,
                              role: v,
                              content: '',
                              key: nanoid(),
                            };
                          }
                          return { ...it, role: v };
                        }
                        return it;
                      });
                      return newInfo as any;
                    });
                  }}
                  onMessageChange={v => {
                    setMessageList(prev => {
                      const newInfo = prev?.map(it => {
                        if (it.key === v.key) {
                          const { parts } = v;
                          return {
                            ...it,
                            ...v,
                            content: parts?.length ? '' : v.content,
                          };
                        }
                        return it;
                      });

                      return newInfo as any;
                    });
                  }}
                  minHeight={26}
                  showDragBtn
                  onDelete={delKey => {
                    setMessageList(prev => {
                      const newInfo = prev?.filter(it => it.key !== delKey);
                      return newInfo;
                    });
                  }}
                  optimizeBtnHidden={!isFirstSp}
                  isJinja2Template={templateType === TemplateType.Jinja2}
                  modalVariableEnable={currentModel?.ability?.multi_modal}
                  modalVariableBtnHidden={message.role === Role.System}
                >
                  <LibraryBlockWidget librarys={librarys} />
                </LoopPromptEditor>
              );
            })}
        </div>

        <Button
          color="primary"
          icon={<IconCozPlus />}
          onClick={handleAddMessage}
          disabled={readonly}
        >
          添加消息
        </Button>
      </div>
    </CollapseCard>
  );
}
