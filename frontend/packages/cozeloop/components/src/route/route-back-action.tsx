// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLocation, type Location } from 'react-router-dom';

import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { IconCozLongArrowUp } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

function getBackRoute(defaultRoute: string, location: Location) {
  // 如果跳转时标明了来源，就返回来源页面路径
  if (location.state?.from) {
    // return location.state.from;
    return -1;
  }
  const performanceEntries = window.performance.getEntriesByType('navigation');
  const navigationEntry = performanceEntries[0] as unknown as
    | { type: string }
    | undefined;
  const isNewNavigation = navigationEntry?.type === 'navigate';
  return isNewNavigation ? defaultRoute : -1;
}

interface RouteBackActionProps {
  onBack?: () => void;
  defaultModuleRoute?: string;
}

export default function RouteBackAction({
  onBack,
  defaultModuleRoute,
}: RouteBackActionProps) {
  const navigateModule = useNavigateModule();
  const location = useLocation();
  return (
    <Button color="secondary" className="!w-[32px] !h-[32px]">
      <IconCozLongArrowUp
        className="-rotate-90 text-[20px] cursor-pointer shrink-0 !coz-fg-plus !font-medium"
        onClick={() => {
          if (typeof onBack === 'function') {
            onBack();
            return;
          }
          const backRoute = getBackRoute(defaultModuleRoute ?? '', location);
          navigateModule(backRoute as string);
        }}
      />
    </Button>
  );
}
