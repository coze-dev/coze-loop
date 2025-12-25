// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { BlackSchemaEditorGroup } from '@cozeloop/evaluate-components';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';

import { BlackDetailHeader } from '@/components/evaluator-ecosystem/black-detail-header';

interface PresetBlackCodeDetailProps {
  evaluator?: Evaluator;
}

export function PresetBlackCodeDetail({
  evaluator,
}: PresetBlackCodeDetailProps) {
  const blackBoxCase = {
    inputValue: JSON.stringify(
      evaluator?.current_version?.evaluator_content?.input_schemas?.reduce(
        (acc: Record<string, string>, cur) => {
          if (cur.key) {
            acc[cur.key] = '';
          }
          return acc;
        },
        {},
      ) ?? {},
      null,
      2,
    ),
    outputValue: JSON.stringify(
      evaluator?.current_version?.evaluator_content?.output_schemas?.reduce(
        (acc: Record<string, string>, cur) => {
          if (cur.key) {
            acc[cur.key] = '';
          }
          return acc;
        },
        {},
      ) ?? {},
      null,
      2,
    ),
  };

  const handleClickDebugBtn = () => {
    sendEvent(EVENT_NAMES.cozeloop_pre_evaluator_test, {
      pre_evaluator_card_name: evaluator?.name,
    });
  };

  return (
    <div className="h-full w-full flex flex-col">
      <BlackDetailHeader
        evaluator={evaluator}
        onClickDebugBtn={handleClickDebugBtn}
      />
      <div className="flex-1 overflow-y-auto p-4 styled-scrollbar">
        <BlackSchemaEditorGroup value={blackBoxCase} disabled />
      </div>
    </div>
  );
}
