// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozIllusDone,
  IconCozIllusDoneDark,
} from '@coze-arch/coze-design/illustrations';
import { EmptyState, Modal, Typography } from '@coze-arch/coze-design';

export function showSubmitSuccess(
  onTraceJumpClick: () => void,
  onEvaluateJumpClick: () => void,
) {
  const modal = Modal.info({
    title: I18n.t('submit_new_version'),
    width: 960,
    closable: true,
    content: (
      <div className="w-full h-[470px] flex items-center justify-center">
        <EmptyState
          icon={<IconCozIllusDone width="160" height="160" />}
          darkModeIcon={<IconCozIllusDoneDark width="160" height="160" />}
          title={
            <Typography.Title heading={5} className="!my-4">
              {I18n.t('submitted_successfully')}
            </Typography.Title>
          }
          description={
            <div className="flex flex-col items-center gap-2 w-[400px]">
              <Typography.Text className="flex gap-2 items-center">
                {I18n.t('cozeloop_sdk_data_report_observation')}
                <Typography.Text
                  link
                  onClick={() => {
                    onTraceJumpClick();
                    modal.destroy();
                  }}
                >
                  {I18n.t('go_immediately')}
                </Typography.Text>
              </Typography.Text>
              <Typography.Text className="flex gap-2 items-center">
                {I18n.t('prompt_effect_evaluation')}
                <Typography.Text
                  link
                  onClick={() => {
                    onEvaluateJumpClick();
                    modal.destroy();
                  }}
                >
                  {I18n.t('go_immediately')}
                </Typography.Text>
              </Typography.Text>
            </div>
          }
        />
      </div>
    ),

    okText: I18n.t('close'),
  });
}
