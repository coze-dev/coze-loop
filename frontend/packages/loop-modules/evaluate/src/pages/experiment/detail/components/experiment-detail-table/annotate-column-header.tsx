// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { type ColumnAnnotation } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Button, Modal, Tag } from '@coze-arch/coze-design';

interface Props {
  annotation: ColumnAnnotation;
  spaceID: string;
  experimentID: string;
  onDelete: () => void;
}
export function AnnotateColumnHeader({
  annotation,
  spaceID,
  experimentID,
  onDelete,
}: Props) {
  const removeTag = useRequest(
    () =>
      StoneEvaluationApi.DeleteAnnotationTag({
        workspace_id: spaceID,
        expt_id: experimentID,
        tag_key_id: annotation.tag_key_id,
      }),
    {
      manual: true,
    },
  );

  return (
    <div className="group flex items-center max-w-full">
      <TypographyText>{annotation.tag_key_name}</TypographyText>
      <Tag color="grey" size="small" className="ml-1 shrink-0">
        {I18n.t('manual_annotation')}
      </Tag>
      <Button
        icon={<IconCozTrashCan />}
        size="mini"
        className="ml-1 !w-[20px] !h-[20px] !hidden group-hover:!inline-flex"
        color="secondary"
        onClick={() => {
          Modal.warning({
            title: I18n.t('delete_this_tag'),
            content: I18n.t('deleting_tag_affects_labeled_content'),
            cancelText: I18n.t('cancel'),
            okText: I18n.t('global_btn_confirm'),
            autoLoading: true,
            onOk: async () => {
              await removeTag.runAsync();
              onDelete?.();
            },
          });
        }}
        loading={removeTag.loading}
      />
    </div>
  );
}
