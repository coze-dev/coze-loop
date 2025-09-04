/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useEffect, useMemo } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useResourcePageJump, useSpace } from '@cozeloop/biz-hooks-adapter';
import { tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

const MAX_RENDER_TAG = 100;

interface TagSelectProps extends SelectProps {
  showDisableTag?: boolean;
  onChangeWithObject?: boolean;
  disableSelectList?: string[];
  defaultShowName?: string;
  showCreateTagButton?: boolean;
  hidedRepeatTags?: boolean;
}

export function TagSelect(props: TagSelectProps) {
  const {
    showDisableTag = false,
    onChangeWithObject = false,
    disableSelectList,
    defaultShowName = '',
    showCreateTagButton = false,
    hidedRepeatTags = false,
    ...rest
  } = props;
  const { spaceID } = useSpace();
  const { getTagCreateURL } = useResourcePageJump();

  const { loading, data, run } = useRequest(
    async (text?: string) => {
      const res = await DataApi.SearchTags({
        tag_key_name_like: text || undefined,
        workspace_id: spaceID,
        page_size: MAX_RENDER_TAG,
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
                <Tag color="primary">{I18n.t('disable')}</Tag>
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

  const optionsList = hidedRepeatTags
    ? data?.filter(item => {
        if (item.value === value?.value) {
          return true;
        }

        return !disableSelectList?.includes(item.value ?? '');
      })
    : data;

  return (
    <BaseSearchSelect
      placeholder={I18n.t('tag_name')}
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
        showCreateTagButton && (
          <div
            className="flex items-center gap-x-2 text-brand-9 h-8 p-1 font-medium cursor-pointer"
            onClick={() => {
              sendEvent(
                EVENT_NAMES.cozeloop_observation_manual_feedback_add_label,
                {
                  space_id: spaceID,
                },
              );
              window.open(getTagCreateURL());
            }}
          >
            <IconCozPlus className="w-4 h-4" />
            <span className="text-[14px] ">{I18n.t('create_tag')}</span>
          </div>
        )
      }
    />
  );
}
