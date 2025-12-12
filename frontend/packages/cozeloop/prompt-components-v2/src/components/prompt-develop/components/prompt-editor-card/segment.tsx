import { useCallback, useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { I18n } from '@cozeloop/i18n-adapter';
import { handleCopy } from '@cozeloop/components';
import {
  Role,
  TemplateType,
  type VariableDef,
} from '@cozeloop/api-schema/prompt';
import {
  IconCozCopy,
  IconCozExpand,
  IconCozMinimize,
} from '@coze-arch/coze-design/icons';
import { IconButton, Tooltip } from '@coze-arch/coze-design';

import { getMockVariables } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { SegmentEditorAction } from '@/components/prompt-editor/widgets/sgement/segment-editor-action';
import { type PromptMessage } from '@/components/prompt-editor/type';
import { PromptEditor } from '@/components/prompt-editor';
import { PromptDevLayout } from '@/components/prompt-dev-layout';
import { type BasicPromptEditorRef } from '@/components/basic-prompt-editor';

import { usePromptDevProviderContext } from '../prompt-provider';
import { TemplateSelect } from './template-select';

export function SegmentEditorCard() {
  const editorRef = useRef<BasicPromptEditorRef>(null);
  const { renderTemplateType } = usePromptDevProviderContext();

  const {
    promptInfo,
    messageList,
    setMessageList,
    variables,
    templateType,
    setVariables,
  } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      templateType: state.templateType,
      messageList: state.messageList,
      setMessageList: state.setMessageList,
      variables: state.variables,
      setVariables: state.setVariables,
    })),
  );

  const { mockVariables, setMockVariables } = usePromptMockDataStore(
    useShallow(state => ({
      mockVariables: state.mockVariables,
      setMockVariables: state.setMockVariables,
    })),
  );

  const message: PromptMessage | undefined = messageList?.[0];

  const { readonly } = useBasicStore(
    useShallow(state => ({
      streaming: state.streaming,
      setStreaming: state.setStreaming,
      readonly: state.readonly,
    })),
  );

  const [isFullscreen, setIsFullscreen] = useState(false);

  const afterInsertSnippet = useCallback(
    (v?: string, newVariables?: VariableDef[]) => {
      v && editorRef?.current?.insertText?.(v);
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
    setMessageList([...(messageList || [])]);
  }, [templateType?.type]);

  return (
    <PromptDevLayout
      title={I18n.t('prompt_prompt_snippet')}
      actionBtns={
        <div className="flex gap-2">
          <SegmentEditorAction
            disabled={readonly}
            buttonColor="primary"
            afterInsert={afterInsertSnippet}
          />

          {renderTemplateType?.({
            prompt: promptInfo,
            streaming: readonly,
          }) ?? <TemplateSelect streaming={readonly} color="primary" />}
          <Tooltip content={I18n.t('copy')}>
            <IconButton
              icon={<IconCozCopy />}
              color="secondary"
              size="mini"
              onClick={() => {
                const info = message?.content ?? '';
                handleCopy(info);
              }}
            />
          </Tooltip>
          <Tooltip
            content={
              isFullscreen
                ? I18n.t('prompt_exit_fullscreen')
                : I18n.t('evaluate_full_screen')
            }
          >
            <IconButton
              icon={
                isFullscreen ? (
                  <IconCozMinimize fontSize={12} />
                ) : (
                  <IconCozExpand fontSize={12} />
                )
              }
              color="secondary"
              size="mini"
              onClick={() => setIsFullscreen(pre => !pre)}
            />
          </Tooltip>
        </div>
      }
    >
      <div className="px-6 pb-4 overflow-hidden h-full">
        <PromptEditor
          ref={editorRef}
          key={`${message?.key}-${templateType?.value}`}
          message={message}
          onMessageChange={v => {
            setMessageList([
              {
                ...v,
                role: Role.Assistant,
                content: v.content,
              },
            ]);
          }}
          variables={variables}
          hideActionWrap={!isFullscreen}
          isFullscreen={isFullscreen}
          modalVariableBtnHidden
          dragBtnHidden
          messageTypeDisabled
          messageTypeList={[{ value: Role.Assistant, label: 'Snippet' }]}
          onIsFullscreenChange={setIsFullscreen}
          className="h-full"
          isJinja2Template={templateType?.type === TemplateType.Jinja2}
          isGoTemplate={templateType?.type === TemplateType.GoTemplate}
          disabled={readonly}
        />
      </div>
    </PromptDevLayout>
  );
}
