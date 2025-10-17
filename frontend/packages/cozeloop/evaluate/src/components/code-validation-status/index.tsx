/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

{
  /* start_aigc */
}
import React, { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
  IconCozLoading,
} from '@coze-arch/coze-design/icons';

interface ValidationResult {
  valid?: boolean;
  error_message?: string;
}

interface CodeValidationStatusProps {
  validationResult: ValidationResult | null;
  loading: boolean;
}

const contentTextStyle = {
  color: 'var(--coz-fg-secondary, rgba(32, 41, 69, 0.62))',
  fontSize: '14px',
  fontWeight: 500,
};

export function CodeValidationStatus({
  validationResult,
  loading,
}: CodeValidationStatusProps) {
  const content = useMemo(() => {
    if (!validationResult) {
      return null;
    }
    if (loading) {
      return (
        <>
          <div className="flex items-center">
            <IconCozLoading
              className="w-6 h-6 animate-spin mr-2"
              color="var(--coz-fg-dim)"
            />
            <span className="text-sm font-medium text-[16px]">
              代码校验中...
            </span>
          </div>
          <div style={contentTextStyle}>代码语法校验通过后，即可提交</div>
        </>
      );
    }

    if (validationResult.valid) {
      return (
        <>
          <div
            className="flex items-center"
            style={{ color: 'var(--coz-fg-hglt-emerald)' }}
          >
            <IconCozCheckMarkCircleFill className="w-6 h-6 mr-2" />
            <span className="text-sm font-medium text-[16px]">
              代码检查通过
            </span>
          </div>
          <div style={{ ...contentTextStyle, color: 'var(--coz-fg-primary)' }}>
            代码语法正确，可以提交
          </div>
        </>
      );
    }

    return (
      <>
        <div
          className="flex items-center"
          style={{ color: 'var(--coz-fg-hglt-orange)' }}
        >
          <IconCozCrossCircleFill className="w-6 h-6 mr-2" />
          <span className="text-sm font-medium text-[16px]">
            代码检查失败，请重试
          </span>
        </div>
        <div style={contentTextStyle}>
          {validationResult.error_message || I18n.t('evaluate_unknown_error')}
        </div>
      </>
    );
  }, [loading, validationResult]);

  if (!validationResult) {
    return null;
  }

  return (
    <div
      className="min-h-[90px] max-h-[200px] overflow-y-auto p-4 rounded-lg bg-white mt-2 flex flex-col gap-3"
      style={{ border: '1px solid var(--coz-stroke-primary)' }}
    >
      {content}
    </div>
  );
}
{
  /* end_aigc */
}
