// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';
import { useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozCross } from '@coze-arch/coze-design/icons';
import { Button, Spin } from '@coze-arch/coze-design';

import { useFetchTagHistory } from '@/hooks/use-fetch-tag-history';

import { EditHistoryItem } from './edit-history-item';

interface EditHistoryListProps {
  onClose: () => void;
}
export const EditHistoryList = (props: EditHistoryListProps) => {
  const { onClose } = props;
  const editHistoryContainerRef = useRef<HTMLDivElement>(null);
  const { tagId } = useParams();

  const { data, loading } = useFetchTagHistory({
    tagKeyId: tagId ?? '',
    target: editHistoryContainerRef.current,
  });

  return (
    <div className="box-border w-[340px] border-0 border-l border-solid coz-stroke-primary flex flex-col h-full max-h-full min-w-0 overflow-hidden pb-[100px]">
      <div className="h-[48px] px-6 box-border w-full max-w-full min-w-0 flex items-center border-0 border-b border-solid coz-stroke-primary justify-between coz-mg-secondary">
        <div className="text-[14px] font-medium leading-[22px] coz-fg-primary">
          {I18n.t('tag_change_log')}
        </div>
        <Button
          icon={<IconCozCross />}
          onClick={onClose}
          color="secondary"
          size="mini"
        />
      </div>

      {loading ? (
        <div className="p-6 flex flex-col gap-y-3 h-full max-h-full overflow-auto styled-scroll justify-center items-center">
          <Spin />
        </div>
      ) : (
        <div
          className="p-6 flex flex-col gap-y-3 h-full max-h-full overflow-auto styled-scroll flex-1"
          ref={editHistoryContainerRef}
        >
          {data?.list.map((item, index) => (
            <EditHistoryItem
              updatedAt={item.base_info?.updated_at}
              updatedBy={item.base_info?.updated_by}
              changeLog={item.change_logs}
              key={item.id}
            />
          ))}
        </div>
      )}
    </div>
  );
};
