// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type EvaluatorTemplate } from '@cozeloop/api-schema/evaluation';
import {
  IconCozFireFill,
  IconCozLongArrowTopRight,
} from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

export function EvaluatorTemplateInfo({
  template,
  className,
  providerClassName,
}: {
  template: EvaluatorTemplate;
  className?: string;
  providerClassName?: string;
}) {
  const { popularity, evaluator_info } = template ?? {};
  const { vendor, vendor_url } = evaluator_info || {};
  return (
    <div
      className={classNames(
        'flex items-center gap-4 text-xs coz-fg-dim',
        className,
      )}
    >
      {popularity !== undefined ? (
        <span>
          <IconCozFireFill />：
          {popularity > 1000
            ? `${(popularity / 1000).toFixed(1)}k`
            : popularity}
        </span>
      ) : null}
      <div className={classNames('flex items-center', providerClassName)}>
        <span>
          {I18n.t('evaluator_provider')}：{vendor || '-'}
        </span>
        {vendor_url ? (
          <Button
            size="mini"
            color="secondary"
            icon={<IconCozLongArrowTopRight className="coz-fg-dim" />}
            onClick={e => {
              e.stopPropagation();
              window.open(vendor_url);
            }}
          />
        ) : null}
      </div>
    </div>
  );
}
