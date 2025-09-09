// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type ModelConfig } from '@cozeloop/api-schema/evaluation';

import { EvaluateModelConfigEditor } from '@/components/evaluate-model-config-editor';

export function ModelConfigInfo({ data }: { data?: ModelConfig }) {
  return (
    <>
      <div className="text-sm font-medium coz-fg-primary mb-2">
        {I18n.t('model')}
      </div>
      {data ? (
        <EvaluateModelConfigEditor
          value={data}
          disabled={true}
          popoverProps={{ position: 'bottomRight' }}
        />
      ) : (
        '-'
      )}
    </>
  );
}
