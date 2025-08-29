import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { Typography } from '@coze-arch/coze-design';

interface OperatorProps {
  tagInfo: tag.TagInfo;
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
}

export const Operator = ({ tagInfo, tagListPagePath }: OperatorProps) => {
  const navigate = useNavigateModule();

  return (
    <div
      className="flex items-center justify-end gap-x-3 text-[13px] font-normal leading-[22px]"
      onClick={e => e.stopPropagation()}
    >
      <Typography.Text
        link
        onClick={() => {
          console.log('edit');
          navigate(`${tagListPagePath}/${tagInfo.tag_key_id}`);
        }}
        className="text-inherit font-inherit leading-inherit"
      >
        详情
      </Typography.Text>
    </div>
  );
};
