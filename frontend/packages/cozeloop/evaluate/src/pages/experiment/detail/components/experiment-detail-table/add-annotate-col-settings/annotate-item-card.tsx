// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Modal, Space, Tooltip, Typography } from '@coze-arch/coze-design';

import AnnotateItem from './annotate-item';
interface Props {
  data: tag.TagInfo;
  spaceID: string;
  experimentID: string;
  onDelete?: (data: tag.TagInfo) => void;
}

export function AnnotateItemCard({
  data,
  spaceID,
  experimentID,
  onDelete,
}: Props) {
  const { getTagDetailURL } = useResourcePageJump();
  const removeTag = useRequest(
    (tagID: string) =>
      StoneEvaluationApi.DeleteAnnotationTag({
        workspace_id: spaceID,
        expt_id: experimentID,
        tag_key_id: tagID,
      }),
    {
      manual: true,
    },
  );
  return (
    <div className="border border-solid coz-stroke-primary rounded-[6px]">
      <AnnotateItem
        data={data}
        actions={
          <Space spacing={20} className="ml-6">
            <Tooltip content={I18n.t('detail')} theme="dark">
              <Typography.Text
                link
                onClick={() => {
                  window.open(getTagDetailURL(data.tag_key_id || ''));
                }}
              >
                {I18n.t('detail')}
              </Typography.Text>
            </Tooltip>
            <Tooltip content={I18n.t('delete_tag')} theme="dark">
              <Typography.Text
                link
                onClick={() => {
                  Modal.warning({
                    title: I18n.t('delete_this_tag'),
                    content: I18n.t('deleting_tag_affects_labeled_content'),
                    cancelText: I18n.t('cancel'),
                    okText: I18n.t('confirm'),
                    autoLoading: true,
                    onOk: async () => {
                      await removeTag.runAsync(data.tag_key_id || '');
                      onDelete?.(data);
                    },
                  });
                }}
              >
                {I18n.t('delete')}
              </Typography.Text>
            </Tooltip>
          </Space>
        }
      />
    </div>
  );
}
