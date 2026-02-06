// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Search, type SearchProps } from '@coze-arch/coze-design';

export function ExperimentNameSearch({
  value,
  onChange,
  ...rest
}: {
  value?: string;
  disabled?: boolean;
  onChange?: (value: string) => void;
} & SearchProps) {
  return (
    <div className="w-60">
      <Search
        placeholder={I18n.t('search_name')}
        prefix={<IconCozMagnifier />}
        {...rest}
        className={classNames('!w-full', rest.className)}
        style={{ width: '100%', flexShrink: 0, ...(rest.style ?? {}) }}
        value={value}
        onChange={val => onChange?.(val)}
      />
    </div>
  );
}
