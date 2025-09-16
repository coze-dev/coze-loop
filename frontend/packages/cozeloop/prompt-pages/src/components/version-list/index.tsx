// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useRequest } from 'ahooks';
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { PromptCreate } from '@cozeloop/prompt-components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import { useModalData } from '@cozeloop/base-hooks';
import {
  type Label,
  type CommitInfo,
  type Prompt,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import { IconCozDuplicate, IconCozUpdate } from '@coze-arch/coze-design/icons';
import {
  Button,
  List,
  Modal,
  Space,
  Spin,
  Toast,
} from '@coze-arch/coze-design';

import { sleep } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useVersionList } from '@/hooks/use-version-list';
import { usePrompt } from '@/hooks/use-prompt';
import { CALL_SLEEP_TIME } from '@/consts';

import { VersionLabelModal } from '../version-label';
import VersionItem from './version-item';

export function VersionList() {
  const { spaceID } = useSpace();
  const navigate = useNavigateModule();

  const labelModal = useModalData<{
    labels: Label[];
    version: string;
  }>();

  const { promptInfo } = usePromptStore(
    useShallow(state => ({ promptInfo: state.promptInfo })),
  );
  const {
    setVersionChangeLoading,
    setVersionChangeVisible,
    versionChangeLoading,
  } = useBasicStore(
    useShallow(state => ({
      setVersionChangeLoading: state.setVersionChangeLoading,
      setVersionChangeVisible: state.setVersionChangeVisible,
      versionChangeLoading: state.versionChangeLoading,
    })),
  );
  const [draftVersion, setDraftVersion] = useState<CommitInfo>();

  const { getPromptByVersion } = usePrompt({ promptID: promptInfo?.id });

  const [activeVersion, setActiveVersion] = useState<string | undefined>();

  const [getDraftLoading, setGetDraftLoading] = useState(true);

  const promptInfoModal = useModalData<Prompt>();

  const {
    versionListData,
    versionListLoadMore,
    versionListLoading,
    versionListReload,
    versionListLoadingMore,
    versionListMutate,
  } = useVersionList({
    promptID: promptInfo?.id,
    draftVersion,
  });

  const isActionButtonShow = Boolean(activeVersion);

  const { runAsync: rollbackRunAsync } = useRequest(
    () =>
      StonePromptApi.RevertDraftFromCommit({
        prompt_id: promptInfo?.id,
        commit_version_reverting_from: activeVersion,
      }),
    {
      manual: true,
      ready: Boolean(spaceID && promptInfo?.id && activeVersion),
      refreshDeps: [spaceID, promptInfo?.id, activeVersion],
      onSuccess: async () => {
        Toast.success('回滚成功');
        setVersionChangeLoading(true);
        await sleep(CALL_SLEEP_TIME);
        getPromptByVersion()
          .then(() => {
            setVersionChangeLoading(false);
            setVersionChangeVisible(false);
          })
          .catch(() => {
            setVersionChangeLoading(false);
            setVersionChangeVisible(false);
          });
      },
    },
  );

  const handleVersionChange = (version?: string) => {
    if (version === activeVersion) {
      return;
    }
    setVersionChangeLoading(true);
    getPromptByVersion(version || '', true)
      .then(() => {
        setVersionChangeLoading(false);
        sendEvent(EVENT_NAMES.cozeloop_pe_version, {
          prompt_id: `${promptInfo?.id || 'playground'}`,
        });
      })
      .catch(() => {
        setVersionChangeLoading(false);
      });

    setActiveVersion(version);
  };

  const handleLabelChange = (version: string, labels: Label[]) => {
    if (!versionListData) {
      return;
    }
    const newLabelMap = {
      ...versionListData.versionLabelMap,
    };
    if (!newLabelMap[version]) {
      newLabelMap[version] = [];
    }

    const changeLabelSet = new Set(labels.map(item => item.key));

    for (const [key, value] of Object.entries(newLabelMap)) {
      if (key !== version) {
        newLabelMap[key] = value.filter(item => !changeLabelSet.has(item.key));
      } else {
        newLabelMap[key] = labels;
      }
    }
    versionListMutate({
      ...versionListData,
      versionLabelMap: newLabelMap,
    });
  };

  useEffect(() => {
    if (spaceID && promptInfo?.id) {
      promptInfo?.prompt_draft?.draft_info &&
        setDraftVersion({
          version: '',
          base_version:
            promptInfo?.prompt_draft?.draft_info?.base_version || '',
          description: '',
          committed_by: '',
          committed_at: promptInfo?.prompt_draft?.draft_info?.updated_at,
        });
      setActiveVersion('');
      setGetDraftLoading(false);
      setTimeout(() => {
        versionListReload();
      }, CALL_SLEEP_TIME);
    }
    return () => {
      setActiveVersion(undefined);
      setGetDraftLoading(true);
    };
  }, [spaceID, promptInfo?.id]);

  return (
    <div className="flex-1 w-full h-full py-6 flex flex-col gap-2 overflow-hidden ">
      <div
        className="w-full h-full overflow-y-auto px-6"
        onScroll={e => {
          const target = e.currentTarget;

          const isAtBottom =
            target.scrollHeight - target.scrollTop <= target.clientHeight + 1;

          if (
            !versionListData?.hasMore ||
            !isAtBottom ||
            versionListLoadingMore
          ) {
            return;
          }
          versionListLoadMore();
        }}
      >
        <List
          dataSource={versionListData?.list || []}
          renderItem={item => (
            <VersionItem
              className="cursor-pointer mb-3"
              key={item.version}
              active={activeVersion === item.version}
              version={item}
              labels={
                versionListData?.versionLabelMap?.[item.version || ''] || []
              }
              onClick={() => handleVersionChange(item.version)}
              onEditLabels={v => {
                labelModal.open({
                  labels: v,
                  version: item.version || '',
                });
              }}
            />
          )}
          size="small"
          emptyContent={
            versionListLoading || getDraftLoading ? <div></div> : null
          }
          loadMore={
            versionListLoadingMore || getDraftLoading ? (
              <div className="w-full text-center">
                <Spin />
              </div>
            ) : null
          }
        />
      </div>

      {isActionButtonShow ? (
        <Space className="w-full flex-shrink-0 px-6">
          <Button
            className="flex-1"
            color="primary"
            disabled={versionChangeLoading}
            icon={<IconCozDuplicate />}
            onClick={() => promptInfoModal.open(promptInfo)}
          >
            创建副本
          </Button>
          <Button
            className="flex-1"
            color="red"
            disabled={versionChangeLoading}
            icon={<IconCozUpdate />}
            onClick={() =>
              Modal.confirm({
                title: '还原为此版本',
                content: '还原后将覆盖最新编辑的提示词。确认还原为此版本？',
                onOk: rollbackRunAsync,
                cancelText: '取消',
                okText: '还原',
                okButtonProps: {
                  color: 'red',
                },
                autoLoading: true,
              })
            }
          >
            还原为此版本
          </Button>
        </Space>
      ) : null}
      <PromptCreate
        visible={promptInfoModal.visible}
        onCancel={promptInfoModal.close}
        data={promptInfoModal?.data}
        isCopy
        onOk={res => {
          navigate(`pe/prompts/${res.cloned_prompt_id}`);
          promptInfoModal.close();
        }}
      />
      <VersionLabelModal
        visible={labelModal.visible}
        spaceID={spaceID}
        promptID={promptInfo?.id || ''}
        labels={labelModal.data?.labels || []}
        version={labelModal.data?.version}
        onCancel={() => {
          labelModal.close();
        }}
        onConfirm={val => {
          handleLabelChange(
            labelModal.data?.version || '',
            val.map(item => ({ key: item })),
          );
          labelModal.close();
        }}
      />
    </div>
  );
}
