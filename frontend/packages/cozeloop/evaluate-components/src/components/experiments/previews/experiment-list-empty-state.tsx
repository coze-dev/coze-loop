import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState } from '@coze-arch/coze-design';

export function ExperimentListEmptyState({
  hasFilterCondition,
}: {
  hasFilterCondition: boolean;
}) {
  return (
    <EmptyState
      size="full_screen"
      icon={<IconCozIllusAdd />}
      title={hasFilterCondition ? '未能找到相关结果' : '暂无实验'}
      description={
        hasFilterCondition
          ? '请尝试其他关键词或修改筛选项'
          : '点击右上角新建实验按钮进行创建'
      }
    />
  );
}
