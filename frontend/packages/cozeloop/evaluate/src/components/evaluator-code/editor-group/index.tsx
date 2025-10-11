// start_aigc
import React, { useRef, useState, useCallback } from 'react';

import cls from 'classnames';

import type { EditorGroupProps } from '../types';
import FuncExecutor from './func-executor';
import DataSetConfig from './data-set-config';

export const EditorGroup: React.FC<EditorGroupProps> = props => {
  const { fieldPath = '', disabled, editorHeight } = props;
  const [leftWidth, setLeftWidth] = useState(50); // 左侧宽度百分比
  const [isDragging, setIsDragging] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isDragging || !containerRef.current) {
        return;
      }

      const container = containerRef.current;
      const containerRect = container.getBoundingClientRect();
      const newLeftWidth =
        ((e.clientX - containerRect.left) / containerRect.width) * 100;

      // 限制宽度范围在 30% - 70% 之间
      const clampedWidth = Math.max(30, Math.min(70, newLeftWidth));
      setLeftWidth(clampedWidth);
    },
    [isDragging],
  );

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  // 添加全局鼠标事件监听
  React.useEffect(() => {
    if (isDragging) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';

      return () => {
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      };
    }
  }, [isDragging, handleMouseMove, handleMouseUp]);

  return (
    <div className="flex flex-col h-full bg-white">
      {/* Content - Left and Right Panels */}
      <div
        ref={containerRef}
        className="flex-1 flex overflow-hidden"
        style={{
          minHeight: 400,
          border: '1px solid rgba(82, 100, 154, 0.13)',
          borderRadius: '8px',
        }}
      >
        {/* Left Panel - Function Executor */}
        <div
          className="func-executor-container border-r border-gray-200 overflow-hidden"
          style={{ width: `${leftWidth}%` }}
        >
          <FuncExecutor
            noLabel={true}
            fieldStyle={{ height: '100%' }}
            field={fieldPath ? `${fieldPath}.funcExecutor` : 'funcExecutor'}
            disabled={disabled}
            fieldClassName="h-full !p-0"
            editorHeight={editorHeight}
          />
        </div>

        {/* Resizer */}
        <div
          className={cls(
            'w-[2px] bg-gray-200 cursor-col-resize hover:bg-blue-400 transition-colors',
            {
              'bg-blue-400': isDragging,
            },
          )}
          onMouseDown={handleMouseDown}
        />

        {/* Right Panel - Data Set Config */}
        <div
          className="data-set-config-container overflow-hidden"
          style={{ width: `${100 - leftWidth}%`, minWidth: 340 }}
        >
          <DataSetConfig
            noLabel={true}
            field={fieldPath ? `${fieldPath}.testData` : 'testData'}
            disabled={disabled}
            fieldClassName="h-full !p-0"
          />
        </div>
      </div>
    </div>
  );
};

export default EditorGroup;
// end_aigc
