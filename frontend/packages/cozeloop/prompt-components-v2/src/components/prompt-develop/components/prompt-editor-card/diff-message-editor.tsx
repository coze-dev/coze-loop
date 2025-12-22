// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  Role,
  type VariableDef,
  type Message,
  VariableType,
  TemplateType,
} from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { StonePromptApi } from '@cozeloop/api-schema';
import { Tag, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { PromptVersionSelect } from '@/components/prompt-version-select';
import { type PromptMessage } from '@/components/prompt-editor/type';
import { PromptDiffEditor } from '@/components/prompt-editor/diff';

import { usePromptDevProviderContext } from '../prompt-provider';

interface DiffMessageEditorProps {
  currentModel?: Model;
  disabled?: boolean;
  variables?: VariableDef[];
  cozeLibrarys?: any[];
  prevVersion?: string;
  messageList?: (Message & {
    key?: string;
  })[];
  onMessageChange?: (key?: string, message?: PromptMessage) => void;
  onMessageTypeChange?: (key?: string, role?: Role) => void;
  onDeleteMessage?: (key?: string) => void;
}
export function DiffMessageEditor({
  currentModel,
  prevVersion,
  messageList = [],
  disabled,
  variables,
  cozeLibrarys,
  onMessageChange,
  onMessageTypeChange,
  onDeleteMessage,
}: DiffMessageEditorProps) {
  const {
    spaceID,
    renderEditorLeftActions,
    renderEditorRightActions,
    hideSnippet,
  } = usePromptDevProviderContext();
  const { promptInfo, templateType } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      templateType: state.templateType,
    })),
  );

  const currentVersion = promptInfo?.prompt_draft?.draft_info
    ? ''
    : promptInfo?.prompt_commit?.commit_info?.version;
  const baseVersion =
    promptInfo?.prompt_draft?.draft_info?.base_version ||
    promptInfo?.prompt_commit?.commit_info?.base_version;

  const [diffVersion, setDiffVersion] = useState(prevVersion);

  const getPromptService = useRequest(
    () =>
      StonePromptApi.GetPrompt({
        prompt_id: promptInfo?.id,
        workspace_id: spaceID,
        commit_version: diffVersion,
        with_commit: true,
      }),
    {
      manual: false,
      refreshDeps: [diffVersion],
      ready: Boolean(promptInfo?.id && spaceID && diffVersion),
    },
  );
  const preMessageList =
    getPromptService?.data?.prompt?.prompt_commit?.detail?.prompt_template
      ?.messages || [];

  const foreachMessageList = useMemo(
    () =>
      preMessageList.length > messageList.length ? preMessageList : messageList,
    [preMessageList, messageList],
  );

  return (
    <div className="flex flex-col w-full border border-solid coz-stroke-primary rounded-[6px] pb-1">
      <div className="!h-9 flex w-full overflow-hidden">
        <div className="flex gap-[10px] w-full h-full items-center px-3 coz-bg-plus rounded-tl-[6px]">
          <Typography.Text size="small" className="!font-semibold">
            {I18n.t('prompt_compare_versions')}
          </Typography.Text>
          <PromptVersionSelect
            className="w-[100px]"
            spaceID={spaceID}
            promptID={promptInfo?.id}
            size="small"
            value={diffVersion}
            onChange={v => setDiffVersion(v as string)}
          />

          {diffVersion === baseVersion ? (
            <Tag>{I18n.t('prompt_source_version')}</Tag>
          ) : null}
        </div>
        <div className="w-[6px] h-full border-0 border-solid !border-r coz-stroke-primary flex-shrink-0"></div>
        <div className="flex gap-[10px] w-full h-full items-center pl-2 pr-3">
          <Typography.Text size="small" className="!font-semibold">
            {currentVersion
              ? `${I18n.t('current_version')} ${currentVersion}`
              : I18n.t('prompt_currently_editing_version')}
          </Typography.Text>
        </div>
      </div>
      {foreachMessageList?.map((_, index) => {
        const preMessage = preMessageList?.[index];
        const message = messageList?.[index] || {};
        return (
          <PromptDiffEditor
            className="border !border-t-[var(--coz-stroke-primary)] !border-l-transparent !border-r-transparent !border-b-transparent !rounded-none"
            preMessage={preMessage}
            key={`${getPromptService.loading}-${message.key}-${templateType?.value}-${diffVersion}`}
            message={message}
            variables={variables?.filter(
              it =>
                it.type !== VariableType.Placeholder &&
                it.type !== VariableType.MultiPart,
            )}
            disabled={disabled}
            dragBtnHidden
            onMessageTypeChange={v => onMessageTypeChange?.(message.key, v)}
            onMessageChange={v => onMessageChange?.(message.key, v)}
            minHeight={26}
            onDelete={delMsg => onDeleteMessage?.(delMsg?.key)}
            modalVariableEnable={currentModel?.ability?.multi_modal}
            modalVariableBtnHidden={message.role === Role.System}
            cozeLibrarys={cozeLibrarys}
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
            snippetBtnHidden={hideSnippet}
          />
        );
      })}
    </div>
  );
}
