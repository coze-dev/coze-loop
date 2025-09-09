// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMatch, useParams } from 'react-router-dom';

export type AppType =
  | 'pe'
  | 'evaluation'
  | 'observation'
  | 'model'
  | 'enterprise-manage'
  | 'open';

export function useApp() {
  const { enterpriseID: enterpriseIDFromURL, spaceID: spaceIDFromURL } =
    useParams<{
      enterpriseID: string;
      spaceID: string;
    }>();

  const enterpriseSpace = useMatch(
    '/console/enterprise/:enterpriseID/space/:spaceID/:app/:subModule/:detail?/*',
  );

  const enterpriseCommon = useMatch(
    '/console/enterprise/:enterpriseID/:app/:subModule/:detail?/*',
  );

  let app = '';
  let subModule = '';
  let detail = '';
  if (enterpriseSpace) {
    app = enterpriseSpace.params.app || '';
    subModule = enterpriseSpace.params.subModule || '';
    detail = enterpriseSpace.params.detail || '';
  } else if (enterpriseCommon) {
    if (
      enterpriseCommon.params.app === 'enterprise-manage' ||
      enterpriseCommon.params.app === 'open'
    ) {
      app = enterpriseCommon.params.app || '';
      subModule = enterpriseCommon.params.subModule || '';
      detail = enterpriseCommon.params.detail || '';
    }
  }

  /**
   * 路径规范 /console/enterprise/:enterpriseID/space/:spaceID/:app/:subModule/:other
   * 路径规范2 /console/enterprise/:enterpriseID/enterprise-manage/:subModule
   * other 可能是创建页面，也可能是详情页
   */

  return {
    spaceIDFromURL,
    enterpriseIDFromURL,
    app: app as AppType,
    subModule,
    // 是否在模块一级页面
    isTopLevel: !detail,
    // 如果是数字的话，则认为在详情页
    isDetail: /^\d+$/.test(detail),
    inSpace: !!enterpriseSpace,
  };
}
