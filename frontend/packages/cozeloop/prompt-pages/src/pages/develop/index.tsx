// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';
import { useState } from 'react';

import {
  PromptDevelop,
  showSubmitSuccess,
} from '@cozeloop/prompt-components-v2';
import { useBreadcrumb, useModalData } from '@cozeloop/hooks';
import { useReportEvent } from '@cozeloop/components';
import {
  useModelList,
  useNavigateModule,
  useOpenWindow,
  useSpace,
} from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import { type Prompt } from '@cozeloop/api-schema/prompt';

import { TraceTab } from '@/components/trace-tabs';
import { ExecuteHistoryPanel } from '@/components/execute-history-panel';

export default function PromptDevelopPage() {
  const sendEvent = useReportEvent();
  const { promptID } = useParams<{
    promptID: string;
  }>();
  const { spaceID } = useSpace();

  const [promptInfo, setPromptInfo] = useState<Prompt>();
  const traceHistoryPannel = useModalData();
  const traceLogPannel = useModalData<string>();
  const navigate = useNavigateModule();
  const { openBlank } = useOpenWindow();

  useBreadcrumb({
    text: promptInfo?.prompt_basic?.display_name || '',
  });

  const service = useModelList(spaceID);

  return (
    <>
      <PromptDevelop
        bizID="CozeLoop"
        spaceID={spaceID}
        promptID={promptID}
        onPromptLoaded={setPromptInfo}
        modelInfo={{
          list: service.data?.models,
          loading: service.loading,
        }}
        sendEvent={sendEvent}
        multiModalConfig={{
          imageSupported: true,
          intranetUrlValidator: url => url.includes('localhost'),
        }}
        canDiffEdit={false}
        debugAreaConfig={{
          hideRoleChange: true,
          canEditMessageType: false,
        }}
        uploadFile={uploadFile}
        buttonConfig={{
          traceHistoryButton: {
            onClick: () => traceHistoryPannel.open(),
          },
          traceLogButton: {
            onClick: ({ debugId }) => traceLogPannel.open(debugId as string),
          },
          copyButton: {
            onSuccess: ({ prompt }) => openBlank(`pe/prompts/${prompt?.id}`),
          },
          deleteButton: {
            onSuccess: () => navigate('/pe/prompts'),
          },
        }}
        onSubmitSuccess={() => {
          showSubmitSuccess(
            () => navigate('observation/traces'),
            () => navigate('evaluation/datasets'),
          );
        }}
        hideSnippet={true}
      />
      <TraceTab
        displayType="drawer"
        debugID={traceLogPannel.data}
        drawerVisible={traceLogPannel.visible}
        drawerClose={traceLogPannel.close}
      />
      <ExecuteHistoryPanel
        promptID={promptInfo?.id}
        visible={traceHistoryPannel.visible}
        onCancel={traceHistoryPannel.close}
      />
    </>
  );
}
