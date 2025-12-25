// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { debounce } from 'lodash-es';
import classNames from 'classnames';
import { useInfiniteScroll } from 'ahooks';
import { handleScrollToBottom, SLEEP_TIME } from '@cozeloop/toolkit';
import { useI18n } from '@cozeloop/components';
import {
  PromptType,
  type VariableDef,
  type Prompt,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  IconCozIllusEmpty,
  IconCozIllusEmptyDark,
} from '@coze-arch/coze-design/illustrations';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import {
  Button,
  EmptyState,
  Popover,
  Spin,
  Typography,
  type ButtonColor,
  Search,
} from '@coze-arch/coze-design';

import { addVariablesInMap } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptDevProviderContext } from '@/components/prompt-develop/components/prompt-provider';

import { SegmentVersionChange } from './segment-version-change';

interface SegmentEditorActionProps {
  afterInsert?: (key?: string, variables?: VariableDef[]) => void;
  disabled?: boolean;
  buttonColor?: ButtonColor;
}

const SegmentListContent = ({
  segmentList,
  loadMoreSegmentList,
  segmentListLoading,
  segmentListNoMore,
  segmentListLoadingMore,
  onSegmentSelected,
}: {
  segmentList?: Prompt[];
  loadMoreSegmentList?: () => void;
  segmentListLoading?: boolean;
  segmentListLoadingMore?: boolean;
  segmentListNoMore?: boolean;
  onSegmentSelected?: (prompt?: Prompt) => void;
}) => {
  const [promptInfo, setPromptInfo] = useState<Prompt>();

  useEffect(() => {
    if (!segmentListLoading && segmentList?.length) {
      setPromptInfo(segmentList[0]);
    } else {
      setPromptInfo(undefined);
    }
  }, [segmentListLoading, segmentList]);

  return segmentListLoading ? (
    <Spin wrapperClassName="w-full h-full flex flex-1 justify-center items-center" />
  ) : (
    <div className="w-full h-full flex flex-1 overflow-hidden">
      <div
        className="min-w-[152px] w-2/3 h-full overflow-y-auto styled-scrollbar py-1 flex flex-col gap-0.5"
        onScroll={e => {
          if (segmentListNoMore) {
            return;
          }
          loadMoreSegmentList && handleScrollToBottom(e, loadMoreSegmentList);
        }}
        style={{
          borderRight:
            '1px solid var(--Stroke-COZ-stroke-primary, rgba(82, 100, 154, 13%))',
        }}
      >
        {segmentList?.map(item => (
          <div
            key={item.id}
            className={classNames(
              'w-full px-2 py-1.5 flex items-center gap-1 cursor-pointer hover:coz-mg-primary-hovered rounded-[6px] overflow-hidden',
              {
                'coz-mg-primary-hovered': promptInfo?.id === item?.id,
              },
            )}
            onClick={() => {
              setPromptInfo(item);
            }}
          >
            <Typography.Text
              className="flex-1"
              ellipsis={{ showTooltip: true }}
            >
              {item?.prompt_basic?.display_name}
            </Typography.Text>
          </div>
        ))}

        {segmentListLoadingMore ? (
          <Spin wrapperClassName="w-full h-8 text-center" size="small" />
        ) : null}
      </div>
      <SegmentVersionChange
        segmentInfo={promptInfo}
        onItemClick={v => onSegmentSelected?.(v)}
      />
    </div>
  );
};

export function SegmentEditorAction({
  afterInsert,
  disabled,
  buttonColor = 'secondary',
}: SegmentEditorActionProps) {
  const [dropVisible, setDropVisible] = useState(false);
  const I18n = useI18n();
  const { spaceID } = usePromptDevProviderContext();
  const { promptInfo } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
    })),
  );

  const segmentEmpty = (
    <EmptyState
      className="flex-1 flex flex-col items-center justify-center max-w-full h-full"
      icon={<IconCozIllusEmpty className="w-16 h-16" />}
      darkModeIcon={<IconCozIllusEmptyDark className="w-16 h-16" />}
      title={I18n.t('prompt_no_content')}
    />
  );

  const { setSnippetMap } = usePromptStore(
    useShallow(state => ({
      setSnippetMap: state.setSnippetMap,
    })),
  );

  const [serachText, setSearchText] = useState<string>();

  const segmentListService = useInfiniteScroll<{
    list: Prompt[];
    nextId: Int64;
  }>(
    async d => {
      const listRes = spaceID
        ? await StonePromptApi.ListPrompt({
            page_num: Number(d?.nextId || 1),
            page_size: 10,
            filter_prompt_types: [PromptType.Snippet],
            workspace_id: spaceID!,
            committed_only: true,
            key_word: serachText || undefined,
          })
        : { prompts: [] };
      const list =
        listRes.prompts?.filter(item => item.id !== promptInfo?.id) || [];
      const total = listRes.total || 0;
      const nId =
        total > (d?.list?.length || 0) ? Number(d?.nextId || 1) + 1 : '0';
      return new Promise<{ list: Prompt[]; nextId: Int64 }>(resolve => {
        resolve({
          list,
          nextId: nId,
        });
      });
    },
    {
      reloadDeps: [serachText],
      isNoMore: d => d?.nextId === '0',
      manual: true,
    },
  );

  const debounceSearch = debounce((v?: string) => {
    setSearchText(v);
  }, SLEEP_TIME);

  return (
    <Popover
      className="rounded-[4px] !p-2"
      stopPropagation
      trigger="custom"
      visible={dropVisible}
      content={
        <div
          className="flex flex-col gap-1"
          style={{ width: 265, height: 176 }}
        >
          <>
            <Search
              className="!w-full"
              placeholder={I18n.t('prompt_name_query')}
              onChange={debounceSearch}
            />

            {!segmentListService?.data?.list?.length &&
            !segmentListService.loading ? (
              segmentEmpty
            ) : (
              <>
                <SegmentListContent
                  segmentList={segmentListService?.data?.list}
                  loadMoreSegmentList={segmentListService.loadMoreAsync}
                  segmentListLoading={segmentListService.loading}
                  segmentListLoadingMore={segmentListService.loadingMore}
                  segmentListNoMore={segmentListService.noMore}
                  onSegmentSelected={versionPrompt => {
                    if (!versionPrompt) {
                      return;
                    }
                    const version =
                      versionPrompt.prompt_commit?.commit_info?.version;
                    const text = `<fornax_prompt>id=${versionPrompt?.id}&version=${version}</fornax_prompt>`;
                    setSnippetMap(map => {
                      if (!map) {
                        return {
                          [text]: versionPrompt,
                        };
                      }
                      return {
                        ...map,
                        [text]: versionPrompt,
                      };
                    });

                    const newVariables =
                      versionPrompt?.prompt_commit?.detail?.prompt_template
                        ?.variable_defs || [];

                    newVariables.forEach(v => {
                      v.key && addVariablesInMap(v.key || '', text);
                    });

                    afterInsert?.(text, newVariables);
                    setDropVisible(false);
                  }}
                />
              </>
            )}
          </>
        </div>
      }
      onVisibleChange={v => {
        if (v) {
          segmentListService.reload();
        } else {
          setSearchText(undefined);
        }
        setDropVisible(v);
      }}
      onClickOutSide={() => setDropVisible(false)}
    >
      <Button
        className="!font-medium"
        size="mini"
        color={buttonColor}
        icon={<IconCozArrowDown />}
        iconPosition="right"
        onClick={() => setDropVisible(true)}
        disabled={disabled}
      >
        {I18n.t('prompt_insert_snippet')}
      </Button>
    </Popover>
  );
}
