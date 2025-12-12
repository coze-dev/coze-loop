/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect } from 'react';

import cls from 'classnames';
import { Select, Skeleton, type SelectProps } from '@coze-arch/coze-design';

import { type Left } from '@/shared/components/analytics-logic-expr/logic-expr';
import { useBatchGetTags } from '@/features/trace-list/hooks/use-batch-get-tags';

import styles from '../prompt-select/index.module.less';

export interface CategoricalSelectProps extends SelectProps {
  filterPlayground?: boolean;
  className?: string;
  left: Left;
  customParams: Record<string, any>;
}

export const CategoricalSelect = (props: CategoricalSelectProps) => {
  const { left, className, customParams } = props;
  const service = useBatchGetTags(customParams);

  useEffect(() => {
    if (!left?.extraInfo?.tag_key_id && !left?.extra_info?.tag_key_id) {
      return;
    }

    service.runAsync([
      left?.extra_info?.tag_key_id ?? left?.extraInfo?.tag_key_id ?? '',
    ]);
  }, [left?.extraInfo?.tag_key_id, left?.extra_info?.tag_key_id]);

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
      className={cls('w-96', className)}
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
