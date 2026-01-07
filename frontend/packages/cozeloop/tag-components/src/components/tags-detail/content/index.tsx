// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef, forwardRef, useImperativeHandle } from 'react';

import { type tag, type GetTagSpecResponse } from '@cozeloop/api-schema/data';

import { formatTagDetailToFormValues } from '@/utils';
import {
  type FormValues,
  TagsForm,
  type TagFormRef,
} from '@/components/tags-form';

// 内容组件
interface TagDetailContentProps {
  tagDetail?: tag.TagInfo;
  tagSpec?: GetTagSpecResponse;
  onValueChange: (values: FormValues) => void;
  onSubmit: (values: FormValues) => void;
}

export interface TagDetailContentRef {
  submit: () => void;
}

export const TagDetailContent = forwardRef<
  TagDetailContentRef,
  TagDetailContentProps
>(({ tagDetail, tagSpec, onValueChange, onSubmit }, ref) => {
  const tagFormRef = useRef<TagFormRef>(null);

  useImperativeHandle(ref, () => ({
    submit: () => {
      tagFormRef.current?.submit();
    },
  }));

  return (
    <div className="h-full max-h-full overflow-auto styled-scroll flex-1">
      <div className="max-w-[800px] flex justify-center w-full pt-6 pb-14 mx-auto">
        <TagsForm
          maxTags={tagSpec?.max_total}
          ref={tagFormRef}
          entry="edit-tag"
          onValueChange={onValueChange}
          defaultValues={formatTagDetailToFormValues(tagDetail || {})}
          onSubmit={onSubmit}
        />
      </div>
    </div>
  );
});

TagDetailContent.displayName = 'TagDetailContent';
