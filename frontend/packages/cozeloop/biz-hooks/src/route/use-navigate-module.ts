import { createUseNavigateModule } from '@cozeloop/route-base';

import { useRouteInfo } from './use-route-info';

export const useNavigateModule = createUseNavigateModule(useRouteInfo);
