// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSet,
  type EvalTarget,
} from '@cozeloop/api-schema/evaluation';

import { BaseTargetPreview } from '../base-target-preview';

export default function SetTargetPreview(props: {
  evalTarget: EvalTarget | undefined;
  enableLinkJump?: boolean;
  jumpBtnClassName?: string;
  evalSet?: EvaluationSet;
}) {
  const { evalSet, jumpBtnClassName } = props;
  const { name, evaluation_set_version, id } = evalSet ?? {};
  const { baseURL } = useBaseURL();

  const versionId = evalSet?.evaluation_set_version?.id;

  return (
    <div className="flex">
      <BaseTargetPreview
        name={name ?? '-'}
        version={evaluation_set_version?.version ?? '-'}
        onClick={e => {
          if (!id) {
            return;
          }
          e.stopPropagation();
          window.open(
            `${baseURL}/evaluation/datasets/${id}?version=${versionId}`,
          );
        }}
        enableLinkJump={true}
        jumpBtnClassName={jumpBtnClassName}
      />
    </div>
  );
}
