/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useMemo, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { InfoTooltip, TableWithoutPagination } from '@cozeloop/components';
import {
  type PromptCommitVersions,
  type Prompt,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  IconCozLongArrowTopRight,
  IconCozMagnifier,
} from '@coze-arch/coze-design/icons';
import {
  Input,
  Modal,
  Popover,
  Space,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { PromptVersionSelect } from '../prompt-version-select';

interface SnippetUseageModalProps {
  spaceID: string;
  visible: boolean;
  snippet?: Prompt;
  totalReferenceCount?: number;
  onCancel?: () => void;
  onOk?: () => void;
  onVersionItemClick?: (versionPrompt: Prompt) => void;
}

export function SnippetUseageModal({
  visible,
  snippet,
  totalReferenceCount,
  onCancel,
  onOk,
  spaceID,
  onVersionItemClick,
}: SnippetUseageModalProps) {
  const [commitVersion, setCommitVersion] = useState<string>();
  const [searchText, setSearchText] = useState<string>();

  const service = useRequest(
    () =>
      StonePromptApi.ListParentPrompt({
        workspace_id: spaceID,
        prompt_id: snippet?.id,
        commit_versions: commitVersion ? [commitVersion] : undefined,
      }).then(res => {
        const array = commitVersion
          ? (res.parent_prompts?.[commitVersion] ?? [])
          : [];

        return {
          list: array,
          total: array.length,
        };
      }),
    {
      ready: Boolean(snippet?.id && spaceID && commitVersion),
      refreshDeps: [spaceID, snippet?.id, commitVersion],
    },
  );

  const tableData = useMemo(
    () =>
      (service.data?.list || []).filter(item => {
        if (searchText) {
          return item?.prompt_basic?.display_name?.includes(searchText);
        }
        return true;
      }),
    [service.data, searchText],
  );

  useEffect(() => {
    if (!visible) {
      setCommitVersion(undefined);
      setSearchText(undefined);
    } else {
      setCommitVersion(
        snippet?.prompt_commit?.commit_info?.version ||
          snippet?.prompt_basic?.latest_version,
      );
    }
  }, [visible, snippet]);

  return (
    <Modal
      width={900}
      visible={visible}
      onCancel={onCancel}
      onOk={onOk}
      title={
        <div className="flex gap-1 items-center">
          {I18n.t('prompt_snippet_reference_records')}
          <Tag color="grey">
            {I18n.t('prompt_total_reference_projects', { totalReferenceCount })}
          </Tag>
        </div>
      }
      hasScroll={false}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Typography.Text className="!font-medium flex items-center gap-1">
            {I18n.t('prompt_snippet_version')}
            <InfoTooltip content="" />
          </Typography.Text>
          <PromptVersionSelect
            spaceID={snippet?.workspace_id}
            promptID={snippet?.id}
            value={commitVersion}
            onChange={v => setCommitVersion(v as string)}
          />
        </div>
        <div className="min-h-[388px] max-h-[852px] flex flex-1 w-full">
          <TableWithoutPagination
            tableProps={{
              columns: [
                {
                  title: (
                    <div className="flex items-center gap-1">
                      {I18n.t('prompt_name')}
                      <Popover
                        content={
                          <div className="py-2 px-3">
                            <Input
                              value={searchText}
                              onChange={setSearchText}
                              style={{ width: 280 }}
                            />
                          </div>
                        }
                        trigger="click"
                      >
                        <IconCozMagnifier
                          className={classNames(
                            'hover:coz-fg-hglt cursor-pointer',
                            {
                              'coz-fg-hglt': searchText,
                            },
                          )}
                        />
                      </Popover>
                      {searchText ? (
                        <Tag
                          size="mini"
                          color="brand"
                          closable
                          onClose={() => setSearchText('')}
                        >
                          <Typography.Text
                            size="small"
                            className="text-inherit !max-w-[65px]"
                            ellipsis={{
                              showTooltip: { opts: { theme: 'dark' } },
                            }}
                          >
                            {searchText}
                          </Typography.Text>
                        </Tag>
                      ) : null}
                    </div>
                  ),

                  dataIndex: 'prompt_basic.display_name',
                  key: 'prompt_basic.display_name',
                  width: 230,
                },

                {
                  title: I18n.t('prompt_reference_snippet_prompt_version'),
                  dataIndex: 'commit_versions',
                  key: 'commit_versions',
                  align: 'left',
                  render: (
                    versions: string[],
                    record: PromptCommitVersions,
                  ) => (
                    <Space>
                      {versions.map(version => (
                        <Tag
                          size="mini"
                          color="primary"
                          key={version}
                          suffixIcon={<IconCozLongArrowTopRight />}
                          onClick={() =>
                            onVersionItemClick?.({
                              id: record.id,
                              prompt_key: record.prompt_key,
                              workspace_id: record.workspace_id,
                              prompt_basic: record.prompt_basic,
                              prompt_commit: {
                                commit_info: {
                                  version,
                                },
                              },
                            })
                          }
                        >
                          {version}
                        </Tag>
                      ))}
                    </Space>
                  ),
                },
              ],

              dataSource: tableData,
            }}
          />
        </div>
      </div>
    </Modal>
  );
}
