// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState } from '@coze-arch/coze-design';

export function ExperimentExportListEmptyState() {
  return (
    <EmptyState
      size="full_screen"
      icon={<IconCozIllusAdd />}
      title={I18n.t('no_export_record_yet')}
      description={I18n.t(
        'cozeloop_open_evaluate_click_export_button_top_right',
      )}
    />
  );
}
