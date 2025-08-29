import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState } from '@coze-arch/coze-design';

export function ExperimentExportListEmptyState() {
  return (
    <EmptyState
      size="full_screen"
      icon={<IconCozIllusAdd />}
      title={'暂无导出记录'}
      description={'点击右上角导出按钮进行导出'}
    />
  );
}
