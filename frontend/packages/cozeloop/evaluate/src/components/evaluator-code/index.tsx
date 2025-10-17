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

import { type CommonFieldProps, withField } from '@coze-arch/coze-design';

import type { CodeEvaluatorConfigProps } from './types';
import TrialOperationResults from './trial-operation-results';
import EditorGroup from './editor-group';
import { useEffect, useMemo, useRef, useState } from 'react';

export const BaseCodeEvaluatorConfig: React.FC<
  CodeEvaluatorConfigProps
> = props => {
  const {
    value,
    disabled,
    fieldPath,
    debugLoading,
    resultsClassName,
    editorHeight,
  } = props;
  const { runResults = [] } = value || {};
  // 处理值变更的统一方法

  const editorGroupContainerRef = useRef<HTMLDivElement>(null);
  const [editorGroupHeight, setEditorGroupHeight] = useState<
    number | undefined
  >(); // 初始高度

  useEffect(() => {
    const parent = editorGroupContainerRef.current;
    if (!parent) {
      return;
    }

    // 初始化高度（减去内边距）
    const updateHeight = () => {
      const targetHeight = parent.offsetHeight; // 父容器总高度（含 padding、border）
      const newHeight = targetHeight - 44;
      const diff = Math.abs(newHeight - (editorGroupHeight || 0));

      // 没有高度时，使用当前计算高度
      if (editorGroupHeight === undefined || diff > 18) {
        setEditorGroupHeight(newHeight);
        return;
      }
    };

    // 初始计算
    updateHeight();

    // 监听父容器尺寸变化
    const observer = new ResizeObserver(updateHeight);
    observer.observe(parent);

    // 清理监听
    return () => observer.unobserve(parent);
  }, []);

  const memoizedEditorHeight = useMemo(() => {
    if (editorHeight) {
      return editorHeight;
    }

    return `${editorGroupHeight}px` || '100%';
  }, [editorGroupHeight, editorHeight]);

  return (
    <div className="flex flex-col h-full space-y-4">
      {/* Editor Group */}
      <div
        ref={editorGroupContainerRef}
        className="editor-group-container flex-1 border border-gray-200 rounded-lg overflow-hidden"
      >
        <EditorGroup
          fieldPath={fieldPath}
          disabled={disabled}
          editorHeight={memoizedEditorHeight}
        />
      </div>
      {/* Trial Operation Results */}
      {(runResults && runResults.length > 0) || debugLoading ? (
        <div className="trial-operation-results-container border border-gray-200 rounded-lg overflow-hidden">
          <TrialOperationResults
            results={runResults}
            loading={debugLoading}
            className={resultsClassName}
          />
        </div>
      ) : null}
    </div>
  );
};

// 使用withField包装组件
const CodeEvaluatorConfig: React.ComponentType<
  CodeEvaluatorConfigProps & CommonFieldProps
> = withField(BaseCodeEvaluatorConfig);

export default CodeEvaluatorConfig;
