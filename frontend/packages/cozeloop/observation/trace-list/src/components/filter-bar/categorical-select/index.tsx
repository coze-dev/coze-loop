// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import classNames from 'classnames';
import { useBatchGetTags } from '@cozeloop/tag-components';
import { Select, Skeleton, type SelectProps } from '@coze-arch/coze-design';

import { type Left } from '@/components/logic-expr/logic-expr';

import styles from '../prompt-select/index.module.less';

export interface CategoricalSelectProps extends SelectProps {
  filterPlayground?: boolean;
  className?: string;
  left: Left;
}

export const CategoricalSelect = (props: CategoricalSelectProps) => {
  const { left, className } = props;
  const service = useBatchGetTags();

  useEffect(() => {
    if (!left?.extraInfo?.tag_key_id) {
      return;
    }

    service.runAsync([left?.extraInfo?.tag_key_id ?? '']);
  }, [left?.extraInfo?.tag_key_id]);

  if (props.allowCreate && service.loading) {
    return (
      <Skeleton
        placeholder={<Skeleton.Title className="w-full h-[32px]" />}
        loading
        className="w-full h-[32px]"
      />
    );
  }

  return (
    <Select
      filter
      dropdownClassName={styles['prompt-select-dropdown']}
      loading={service.loading}
      className={classNames('w-96', className)}
      {...props}
      optionList={service.data?.tag_info_list?.[0].tag_values?.map(item => ({
        label: item.tag_value_name ?? '',
        value: item.tag_value_id ?? '',
      }))}
      onChange={v => {
        props.onChange?.(v);
      }}
      value={props.value}
    />
  );
};
