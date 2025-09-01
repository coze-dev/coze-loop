import { useRequest } from 'ahooks';
import { TypographyText } from '@cozeloop/evaluate-components';
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
        人工标注
      </Tag>
      <Button
        icon={<IconCozTrashCan />}
        size="mini"
        className="ml-1 !w-[20px] !h-[20px] !hidden group-hover:!inline-flex"
        color="secondary"
        onClick={() => {
          Modal.warning({
            title: '删除此标签',
            content: '删除此标签将影响已打标的内容',
            cancelText: '取消',
            okText: '确认',
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
