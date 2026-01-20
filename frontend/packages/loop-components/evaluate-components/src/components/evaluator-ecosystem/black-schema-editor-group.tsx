// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import React from 'react';

import './black-schema-editor-group.less';
import { BlackSchemaEditor } from './black-schema-editor';

interface IProps {
  value: {
    inputValue: string;
    outputValue: string;
  };
  onChange?: (value: { inputValue: string; outputValue: string }) => void;
  disabled?: boolean;
  disableLeftPanel?: boolean;
  disableRightPanel?: boolean;
}

export const BlackSchemaEditorGroup: React.FC<IProps> = ({
  value,
  onChange,
  disabled = false,
  disableLeftPanel = false,
  disableRightPanel = false,
}) => {
  const onInputChange = (newInputValue: string) => {
    onChange?.({ ...value, inputValue: newInputValue });
  };

  const onOutputChange = (newOutputValue: string) => {
    onChange?.({ ...value, outputValue: newOutputValue });
  };

  return (
    <div className="black-schema-editor-group">
      <PanelGroup direction="horizontal" className="panel-group">
        <Panel className="panel">
          <BlackSchemaEditor
            title="Input"
            height="500px"
            value={value?.inputValue || ''}
            onChange={onInputChange}
            disabled={disabled || disableLeftPanel}
          />
        </Panel>
        <PanelResizeHandle className="resize-handle-outer">
          <div className="resize-handle-inner" />
        </PanelResizeHandle>
        <Panel className="panel">
          <BlackSchemaEditor
            title="Output"
            height="500px"
            value={value?.outputValue || ''}
            onChange={onOutputChange}
            disabled={disabled || disableRightPanel}
          />
        </Panel>
      </PanelGroup>
    </div>
  );
};
