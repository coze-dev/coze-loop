// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Toast, Tooltip } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { IconButtonContainer } from './icon-button-container';

export function IDRender({
  id,
  showSuffixLength = 5,
  enableCopy = true,
  useTag = false,
  defaultShowCopyBtn,
}: {
  id: Int64;
  showSuffixLength?: number;
  enableCopy?: boolean;
  useTag?: boolean;
  defaultShowCopyBtn?: boolean;
}) {
  const I18n = useI18n();
  const idString = id?.toString() ?? '';
  const suffix = idString.slice(
    Math.max(idString.length - showSuffixLength, 0),
    idString.length,
  );
  return (
    <div
      className="group flex items-center gap-1"
      onClick={e => e.stopPropagation()}
    >
      <Tooltip content={idString} theme="dark">
        {useTag ? (
          <div className="shrink-0 h-5 flex items-center px-2 rounded-[3px] border border-solid border-[var(--coz-stroke-plus)] font-medium text-[var(--coz-fg-primary)]">
            #{suffix || '-'}
          </div>
        ) : (
          <span className="shrink-0">#{suffix || '-'}</span>
        )}
      </Tooltip>
      {enableCopy ? (
        <Tooltip content={I18n.t('copy_id')} theme="dark">
          <div>
            <IconButtonContainer
              className={classNames(
                'id-render-copy-action-button shrink-0 text-sm',
                defaultShowCopyBtn ? '' : ' hidden group-hover:flex',
              )}
              icon={<IconCozCopy />}
              onClick={async e => {
                e.stopPropagation();
                try {
                  await navigator.clipboard.writeText(idString);
                  Toast.success(I18n.t('copy_success'));
                } catch (error) {
                  console.error(error);
                  Toast.error(I18n.t('copy_failed'));
                }
              }}
            />
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}
