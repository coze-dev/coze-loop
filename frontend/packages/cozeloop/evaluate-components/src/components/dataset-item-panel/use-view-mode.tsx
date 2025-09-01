import { useState } from 'react';

import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Button, Select, Typography } from '@coze-arch/coze-design';

export enum ViewMode {
  SLIM = 'slim', // 紧凑视图
  Auto = 'auto', // 聚焦视图，列不限制展示高度
}
const VIEW_MODE_KEY = 'evaluate-dataset-item-view-mode';
export const useViewMode = () => {
  const localStorageViewMode = localStorage.getItem(VIEW_MODE_KEY);
  const initViewMode =
    localStorageViewMode === ViewMode.Auto ? ViewMode.Auto : ViewMode.SLIM;
  const [viewMode, setViewMode] = useState<ViewMode>(initViewMode);

  const getViewModeLabel = (label: ViewMode) => (
    <div className="flex flex-col p-2">
      {label === ViewMode.SLIM ? '紧凑视图' : '宽松视图'}
      <Typography.Text className="text-[12px] coz-fg-secondary">
        {label === ViewMode.SLIM
          ? '在最大高度内滚动查看字段内容'
          : '字段内容不受最大高度限制'}
      </Typography.Text>
    </div>
  );
  const optionList = [
    {
      value: ViewMode.SLIM,
      label: getViewModeLabel(ViewMode.SLIM),
      chipColor: 'secondary',
    },
    {
      value: ViewMode.Auto,
      label: getViewModeLabel(ViewMode.Auto),
      chipColor: 'secondary',
    },
  ];
  const ViewModeNode = (
    <Select
      optionList={optionList}
      value={viewMode}
      triggerRender={props => (
        <Button color="secondary" className="text-[13px] !coz-fg-secondary">
          {viewMode === ViewMode.SLIM ? '紧凑视图' : '宽松视图'}
          <IconCozArrowDown className="ml-1" />
        </Button>
      )}
      onChange={value => {
        setViewMode(value as ViewMode);
        localStorage.setItem(VIEW_MODE_KEY, value as ViewMode);
      }}
    />
  );
  const isAuto = viewMode === ViewMode.Auto;
  return { viewMode, ViewModeNode, isAuto };
};
