// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState, useEffect } from 'react';
import { Card, Button, Input, Alert, Space, Typography } from 'antd';
import { TemplateApi } from '@cozeloop/api-schema';

const { TextArea } = Input;
const { Title, Text } = Typography;

interface TemplatePreviewProps {
  template: string;
  templateType: string;
  variables: Record<string, string>;
  onPreview?: (result: string) => void;
}

export function TemplatePreview({
  template,
  templateType,
  variables,
  onPreview
}: TemplatePreviewProps) {
  const [result, setResult] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [localVariables, setLocalVariables] = useState<Record<string, string>>(variables);

  // 当外部变量变化时更新本地变量
  useEffect(() => {
    setLocalVariables(variables);
  }, [variables]);

  // 当模板或变量变化时自动预览
  useEffect(() => {
    if (template && templateType === 'jinja2') {
      handlePreview();
    }
  }, [template, templateType, localVariables]);

  const handlePreview = async () => {
    if (!template || templateType !== 'jinja2') {
      return;
    }

    setLoading(true);
    setError('');

    try {
      // 调用后端API进行模板预览
      const response = await TemplateApi.previewTemplate({
        template,
        template_type: templateType,
        variables: localVariables,
      });

      if (response.code === 200 && response.data) {
        setResult(response.data.result);
        onPreview?.(response.data.result);
      } else {
        setError(response.msg || 'Preview failed');
      }
    } catch (err: any) {
      setError(err.message || 'Preview failed');
    } finally {
      setLoading(false);
    }
  };

  const handleVariableChange = (key: string, value: string) => {
    setLocalVariables(prev => ({
      ...prev,
      [key]: value,
    }));
  };

  const addVariable = () => {
    const newKey = `var${Object.keys(localVariables).length + 1}`;
    setLocalVariables(prev => ({
      ...prev,
      [newKey]: '',
    }));
  };

  const removeVariable = (key: string) => {
    const newVars = { ...localVariables };
    delete newVars[key];
    setLocalVariables(newVars);
  };

  

  if (templateType !== 'jinja2') {
    return null;
  }

  return (
    <Card
      title={
        <Space>
          <Title level={5} style={{ margin: 0 }}>Template Preview</Title>
          <Button
            size="small"
            onClick={handlePreview}
            loading={loading}
            type="primary"
          >
            Refresh Preview
          </Button>
        </Space>
      }
      style={{ marginTop: 16 }}
    >
      <div style={{ display: 'flex', gap: 16 }}>
        {/* 左侧：变量编辑 */}
        <div style={{ flex: 1 }}>
          <div style={{ marginBottom: 16 }}>
            <Space style={{ marginBottom: 8 }}>
              <Text strong>Variables:</Text>
              <Button size="small" onClick={addVariable}>
                Add Variable
              </Button>
            </Space>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {Object.entries(localVariables).map(([key, value]) => (
              <div key={key} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <Input
                  placeholder="Variable name"
                  value={key}
                  onChange={(e) => {
                    const newVars = { ...localVariables };
                    delete newVars[key];
                    newVars[e.target.value] = value;
                    setLocalVariables(newVars);
                  }}
                  style={{ width: 120 }}
                />
                <Input
                  placeholder="Variable value"
                  value={value}
                  onChange={(e) => handleVariableChange(key, e.target.value)}
                  style={{ flex: 1 }}
                />
                <Button
                  size="small"
                  danger
                  onClick={() => removeVariable(key)}
                >
                  Remove
                </Button>
              </div>
            ))}
          </div>
        </div>

        {/* 右侧：预览结果 */}
        <div style={{ flex: 1 }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>
            Preview Result:
          </Text>

          {error && (
            <Alert
              type="error"
              message={error}
              style={{ marginBottom: 8 }}
            />
          )}

          {result && (
            <div
              style={{
                backgroundColor: '#f8f9fa',
                border: '1px solid #e9ecef',
                borderRadius: 6,
                padding: 12,
                minHeight: 100,
                whiteSpace: 'pre-wrap',
                fontFamily: 'monospace',
                fontSize: 13,
              }}
            >
              {result}
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}
