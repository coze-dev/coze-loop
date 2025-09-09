// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { JumpIconButton } from '@cozeloop/components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { Tag, Tooltip } from '@coze-arch/coze-design';

import { TypographyText } from '../text-ellipsis';

/** 评测集预览 */
export function EvaluationSetPreview({
  evalSet,
  enableLinkJump,
  className,
  jumpBtnClassName,
}: {
  evalSet: EvaluationSet | undefined;
  enableLinkJump?: boolean;
  className?: string;
  jumpBtnClassName?: string;
}) {
  const { name, evaluation_set_version, id } = evalSet ?? {};
  const { baseURL } = useBaseURL();
  if (!evalSet) {
    return <>-</>;
  }
  const versionId = evalSet?.evaluation_set_version?.id;
  return (
    <div
      className={classNames(
        'group inline-flex items-center gap-1 overflow-hidden cursor-pointer max-w-[100%]',
        className,
      )}
      onClick={e => {
        if (enableLinkJump && id) {
          e.stopPropagation();
          window.open(
            `${baseURL}/evaluation/datasets/${id}?version=${versionId}`,
          );
        }
      }}
    >
      <TypographyText>{name ?? '-'}</TypographyText>
      <Tag size="small" color="primary" className="shrink-0">
        {evaluation_set_version?.version ?? '-'}
      </Tag>
      {enableLinkJump ? (
        <Tooltip theme="dark" content={I18n.t('view_detail')}>
          <div>
            <JumpIconButton
              className={classNames(
                'hidden group-hover:flex',
                jumpBtnClassName,
              )}
            />
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}
