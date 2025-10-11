import { createUseOpenWindow, type UseOpenWindow } from '@cozeloop/route-base';

import { useRouteInfo } from './use-route-info';

export const useOpenWindow: UseOpenWindow = createUseOpenWindow(useRouteInfo);
