// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useEffect, useMemo } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { BIZ_EVENTS } from '@/shared/constants';
import { BaseSearchSelect } from '@/shared/components/search-select';
import { useLocale } from '@/i18n';
import { useConfigContext } from '@/config-provider';

const MAX_RENDER_LABEL = 100;

interface LabelSelectProps extends SelectProps {
  showDisableTag?: boolean;
  onChangeWithObject?: boolean;
  disableSelectList?: string[];
  defaultShowName?: string;
  showCreateLabelButton?: boolean;
  hidedRepeatLabels?: boolean;
  customParams: Record<string, any>;
}

export function LabelSelect(props: LabelSelectProps) {
  const {
    showDisableTag = false,
    onChangeWithObject = false,
    disableSelectList,
    defaultShowName = '',
    showCreateLabelButton = false,
    hidedRepeatLabels = false,
    customParams,
    ...rest
  } = props;
  const { t } = useLocale();
  const { sendEvent } = useConfigContext();

  const { loading, data, run } = useRequest(
    async (text?: string) => {
      const res = await DataApi.SearchTags({
        tag_key_name_like: text || undefined,
        workspace_id: customParams?.spaceID ?? '',
        page_size: MAX_RENDER_LABEL,
      });
      return res.tagInfos
        ?.filter(tagInfo => {
          if (showDisableTag) {
            return true;
          }

          if (tagInfo.status !== tag.TagStatus.Inactive) {
            return true;
          }

          return typeof rest.value === 'string'
            ? tagInfo.tag_key_id === rest.value
            : tagInfo.tag_key_id ===
                (rest.value as Record<string, unknown>)?.tag_key_id;
        })
        .map(tagInfo => ({
          value: tagInfo.tag_key_id,
          label: (
            <div className="w-full max-w-full min-w-0 pr-2 flex items-center gap-x-1 justify-start">
              <Typography.Text
                className="max-w-full overflow-hidden !text-inherit"
                style={{ fontSize: 13 }}
                ellipsis={{
                  showTooltip: {
                    opts: {
                      theme: 'dark',
                    },
                  },
                }}
              >
                {tagInfo.tag_key_name}
              </Typography.Text>
              {showDisableTag && tagInfo.status === tag.TagStatus.Inactive ? (
                <Tag color="primary">{t('disabled_label')}</Tag>
              ) : null}
            </div>
          ),
          ...tagInfo,
        }));
    },
    {
      manual: true,
    },
  );

  const handleSearch = useDebounceFn(run, {
    wait: 500,
  });

  useEffect(() => {
    run();
  }, []);

  const renderSelectedItem = useCallback(
    (optionNode?: Record<string, unknown>) =>
      (optionNode?.label || optionNode?.value) as React.ReactNode,
    [],
  );

  const value = useMemo(() => {
    if (loading) {
      return {
        value:
          typeof rest.value === 'string'
            ? rest.value
            : (rest.value as Record<string, unknown>)?.tag_key_id,
        label: (
          <div className="w-full max-w-full min-w-0 pr-2 flex items-center gap-x-1 justify-start">
            <Typography.Text
              className="max-w-full overflow-hidden"
              style={{ fontSize: 13 }}
              ellipsis={{
                showTooltip: {
                  opts: {
                    theme: 'dark',
                  },
                },
              }}
            >
              {defaultShowName}
            </Typography.Text>
          </div>
        ),
      };
    }
    if (typeof rest.value === 'string') {
      return data?.find(item => item.value === rest.value);
    }
    return data?.find(
      item =>
        item.value === (rest.value as Record<string, unknown>)?.tag_key_id,
    );
  }, [rest.value, data, loading, defaultShowName]);

  const optionsList = hidedRepeatLabels
    ? data?.filter(item => {
        if (item.value === value?.value) {
          return true;
        }

        return !disableSelectList?.includes(item.value ?? '');
      })
    : data;

  return (
    <BaseSearchSelect
      placeholder={t('label_name_placeholder')}
      renderSelectedItem={renderSelectedItem as RenderSelectedItemFn}
      {...rest}
      value={value}
      filter
      remote
      loading={loading}
      onSearch={handleSearch.run}
      showRefreshBtn={true}
      onClickRefresh={() => run()}
      optionList={optionsList}
      searchPosition="dropdown"
      multiple={false}
      onChangeWithObject={onChangeWithObject}
      outerBottomSlot={
        showCreateLabelButton && (
          <div
            className="flex items-center gap-x-2 text-brand-9 h-8 p-1 font-medium cursor-pointer"
            onClick={() => {
              sendEvent?.(
                BIZ_EVENTS.cozeloop_observation_manual_feedback_add_label,
                {
                  space_id: customParams?.spaceID ?? '',
                },
              );
              window.open(
                `${customParams?.baseURL ?? ''}/label-management/labels/create`,
                '_blank',
              );
            }}
          >
            <IconCozPlus className="w-4 h-4" />
            <span className="text-[14px] ">{t('create_new_label')}</span>
          </div>
        )
      }
    />
  );
}
