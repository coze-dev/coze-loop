// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { useUserInfo, useSpace } from '@cozeloop/biz-hooks-adapter';
import { StonePromptApi } from '@cozeloop/api-schema';
import { Select, type SelectProps, Skeleton } from '@coze-arch/coze-design';

import styles from './index.module.less';
const PAGE_SIZE = 100;

const reg = /playground-\{\{(\d+)\}\}/;

export interface PromptSelectProps extends SelectProps {
  filterPlayground?: boolean;
  className?: string;
}

export function PromptSelect({
  className,
  filterPlayground = true,
  ...props
}: PromptSelectProps) {
  const { spaceID } = useSpace();
  const userInfo = useUserInfo();
  const service = useRequest(async () => {
    const res = await StonePromptApi.ListPrompt({
      workspace_id: spaceID,
      page_num: 1,
      page_size: PAGE_SIZE,
    });
    return (res.prompts || [])
      .filter(item => {
        const match = item.prompt_key?.match(reg);
        if (!match) {
          return true;
        }
        return match[1] === userInfo?.user_id_str;
      })
      .map(item => {
        const match = item.prompt_key?.match(reg);
        if (!match || !filterPlayground) {
          return {
            label: item.prompt_key,
            value: item.prompt_key || '',
          };
        }
        return {
          label: 'Playground',
          value: item.prompt_key || '',
        };
      })
      .sort((a, b) => {
        if (a.value === props.value) {
          return -1;
        }
        if (b.value === props.value) {
          return 1;
        }
        if (a.label === 'Playground' && filterPlayground) {
          return -1;
        }
        if (b.label === 'Playground' && filterPlayground) {
          return 1;
        }
        return 0;
      });
  });

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
      optionList={service.data || []}
      onChange={v => {
        props.onChange?.(v);
      }}
      value={props.value}
    />
  );
}
