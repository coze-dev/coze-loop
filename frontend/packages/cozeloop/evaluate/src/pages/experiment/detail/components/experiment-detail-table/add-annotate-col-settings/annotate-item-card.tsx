import { useRequest } from 'ahooks';
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
            <Tooltip content="查看详情" theme="dark">
              <Typography.Text
                link
                onClick={() => {
                  window.open(getTagDetailURL(data.tag_key_id || ''));
                }}
              >
                详情
              </Typography.Text>
            </Tooltip>
            <Tooltip content="删除标签" theme="dark">
              <Typography.Text
                link
                onClick={() => {
                  Modal.warning({
                    title: '删除此标签',
                    content: '删除此标签将影响已打标的内容',
                    cancelText: '取消',
                    okText: '确认',
                    autoLoading: true,
                    onOk: async () => {
                      await removeTag.runAsync(data.tag_key_id || '');
                      onDelete?.(data);
                    },
                  });
                }}
              >
                删除
              </Typography.Text>
            </Tooltip>
          </Space>
        }
      />
    </div>
  );
}
