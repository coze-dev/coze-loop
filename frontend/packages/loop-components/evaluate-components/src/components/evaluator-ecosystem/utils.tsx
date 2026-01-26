// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ArgsSchema,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { Tag } from '@coze-arch/coze-design';

const renderTags = (evalTags: Evaluator['tags']) => {
  if (!evalTags) {
    return '-';
  }
  const tagMap = Object.values(evalTags)?.[0] || {};

  delete tagMap.Name;

  // todo: 后面结构修改一同改掉
  const tags = Object.values(tagMap).flat();
  if (tags.length === 0) {
    return '-';
  }

  return tags.map(tag => (
    <Tag key={tag} color="primary" className="mr-1">
      {tag}
    </Tag>
  ));
};

const getSchemaDefaultValueObj = (schemas: ArgsSchema[] = []) =>
  schemas.reduce(
    (prev, cur) => ({
      ...prev,
      [cur.key as string]: cur.default_value,
    }),
    {},
  );

export { renderTags, getSchemaDefaultValueObj };
