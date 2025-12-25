// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Skeleton } from '@coze-arch/coze-design';

export const EvaluatorDetailPlaceholder = (
  <div className="w-full">
    <Skeleton.Title className="w-[240px] mb-4 mt-4" />
    <Skeleton.Paragraph className="flex flex-col gap-2" rows={22} />
  </div>
);
