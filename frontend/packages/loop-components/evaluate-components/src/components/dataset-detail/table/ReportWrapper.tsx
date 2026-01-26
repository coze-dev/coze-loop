// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import { sendEvent } from '@cozeloop/tea-adapter';

interface ReportWrapperProps {
  children: ReactNode;
  reportParams: {
    eventName: string;
    params: Record<string, string>;
  };
}

/** 埋点上报 Wrapper HOC */
const ReportWrapper = (props: ReportWrapperProps) => {
  const { children, reportParams } = props;

  const report = () => {
    sendEvent(reportParams.eventName, reportParams.params);
  };

  return <div onClick={report}>{children}</div>;
};

export default ReportWrapper;
