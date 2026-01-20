// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';

import { PromptDevelop } from '@cozeloop/prompt-components-v2';
import { useModalData } from '@cozeloop/hooks';
import { useReportEvent } from '@cozeloop/components';
import {
  useModelList,
  useNavigateModule,
  useSpace,
} from '@cozeloop/biz-hooks-adapter';

import { TraceTab } from '@/components/trace-tabs';

export default function PromptPlaygroundPage() {
  const navigate = useNavigateModule();
  const sendEvent = useReportEvent();

  const { template } = useParams<{
    template: string;
  }>();
  const { spaceID } = useSpace();
  const service = useModelList(spaceID);
  const traceLogPannel = useModalData<string>();

  return (
    <>
      <PromptDevelop
        spaceID={spaceID}
        promptID={template}
        isPlayground
        modelInfo={{
          list: service.data?.models,
          loading: service.loading,
        }}
        sendEvent={sendEvent}
        buttonConfig={{
          createButton: {
            onSuccess: ({ prompt }) => navigate(`pe/prompts/${prompt?.id}`),
          },
          traceLogButton: {
            onClick: ({ debugId }) => traceLogPannel.open(debugId as string),
          },
        }}
        multiModalConfig={{
          imageSupported: true,
          intranetUrlValidator: url => url.includes('localhost'),
        }}
        hideSnippet={true}
      />
      <TraceTab
        displayType="drawer"
        debugID={traceLogPannel.data}
        drawerVisible={traceLogPannel.visible}
        drawerClose={traceLogPannel.close}
      />
    </>
  );
}
