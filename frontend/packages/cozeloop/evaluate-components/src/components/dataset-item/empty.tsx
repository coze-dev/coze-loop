import { IconCozEmpty } from '@coze-arch/coze-design/icons';

export const EmptyDatasetItem = () => (
  <div className="flex flex-col items-center justify-center h-[80px] p-3 coz-fg-dim coz-bg-plus border border-solid border-[var(--coz-stroke-primary)] rounded-[6px] ">
    <IconCozEmpty className="h-[32px] w-[32px]" />
    <span className="text-[12px]">暂无数据</span>
  </div>
);
