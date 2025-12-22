// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable complexity */
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
import { TemplateType, type Prompt } from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { StonePromptApi } from '@cozeloop/api-schema';
import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';
import {
  CozAvatar,
  IconButton,
  Space,
  Spin,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { convertSnippetsToMap } from '@/utils/prompt';
import {
  getButtonDisabledFromConfig,
  getButtonHiddenFromConfig,
} from '@/utils/base';
import { usePromptStore } from '@/store/use-prompt-store';
import { LABEL_MAP } from '@/consts';

import SegmentCompletion from '../prompt-editor/widgets/sgement/segment-completion';
import { PromptEditor } from '../prompt-editor';
import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';

import styles from './index.module.less';

interface PromptDisplayCardProps {
  promptID?: string;
  promptVersion?: string;
  userDetail?: UserInfoDetail;
}
export function PromptDisplayCard({
  promptID,
  promptVersion,
  userDetail,
}: PromptDisplayCardProps) {
  const [prompt, setPrompt] = useState<Prompt>();
  const [loadingTemplate, setLoadingTemplate] = useState(false);
  const { spaceID, buttonConfig } = usePromptDevProviderContext();

  const { setSnippetMap } = usePromptStore(
    useShallow(state => ({
      setSnippetMap: state.setSnippetMap,
    })),
  );

  useEffect(() => {
    setLoadingTemplate(true);
    if (promptID) {
      StonePromptApi.GetPrompt({
        prompt_id: promptID!,
        workspace_id: spaceID,
        commit_version: promptVersion,
      })
        .then(res => {
          setSnippetMap(map => ({
            ...map,
            ...convertSnippetsToMap(
              res.prompt?.prompt_commit?.detail?.prompt_template?.snippets ||
                [],
            ),
          }));
          setPrompt(res.prompt);
          setLoadingTemplate(false);
        })
        .catch(() => setLoadingTemplate(false));
    } else {
      setLoadingTemplate(false);
    }
  }, [promptID, promptVersion]);

  if (!promptID) {
    return null;
  }

  return (
    <div
      className={classNames(styles['prompt-display-card'], 'styled-scrollbar')}
    >
      <div className={styles['prompt-display-card-header']}>
        <div className="flex flex-1 gap-2">
          <CozAvatar size="plus" src={userDetail?.avatar_url} />
          <div className="flex flex-col">
            <div className="flex gap-2 items-center">
              <Typography.Text
                type="primary"
                className="!font-semibold text-[13px]"
              >
                {userDetail?.name}
              </Typography.Text>
              <Tag color="primary" size="mini">
                {
                  LABEL_MAP[
                    prompt?.prompt_commit?.detail?.prompt_template
                      ?.template_type || TemplateType.Normal
                  ]
                }
              </Tag>
            </div>
            <Typography.Text type="primary" className="text-[13px]">
              {formatTimestampToString(
                prompt?.prompt_commit?.commit_info?.committed_at,
              )}
            </Typography.Text>
          </div>
        </div>
        {getButtonHiddenFromConfig(
          buttonConfig?.snippetJumpButton,
          prompt,
        ) ? null : (
          <IconButton
            icon={<IconCozLongArrowTopRight />}
            color="secondary"
            onClick={() =>
              buttonConfig?.snippetJumpButton?.onClick?.({ prompt })
            }
            disabled={getButtonDisabledFromConfig(
              buttonConfig?.snippetJumpButton,
              prompt,
            )}
          />
        )}
      </div>

      <Typography.Text type="secondary">
        {prompt?.prompt_commit?.commit_info?.description}
      </Typography.Text>

      {prompt?.prompt_commit?.detail?.prompt_template?.variable_defs?.length ? (
        <>
          <Space wrap>
            {prompt?.prompt_commit?.detail?.prompt_template?.variable_defs?.map(
              item => (
                <Tag key={item.key} color="blue">
                  {item.key}
                  {`（${item.type}）`}
                </Tag>
              ),
            )}
          </Space>
        </>
      ) : null}

      {loadingTemplate ? (
        <Spin wrapperClassName="w-full h-full flex items-center justify-center" />
      ) : (
        <div
          onMouseEnter={e => e.stopPropagation()}
          onMouseLeave={e => e.stopPropagation()}
        >
          <PromptEditor
            message={
              prompt?.prompt_commit?.detail?.prompt_template?.messages?.[0]
            }
            disabled
            messageTypeDisabled
            maxHeight={200}
            hideActionWrap
            isJinja2Template={
              prompt?.prompt_commit?.detail?.prompt_template?.template_type ===
              TemplateType.Jinja2
            }
            isGoTemplate={
              prompt?.prompt_commit?.detail?.prompt_template?.template_type ===
              TemplateType.GoTemplate
            }
            linePlaceholder=" "
          >
            <SegmentCompletion />
          </PromptEditor>
        </div>
      )}
    </div>
  );
}
