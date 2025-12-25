// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useMemo, useState } from 'react';

import { safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  handleCopy,
  SchemaEditor,
  TooltipWhenDisabled,
} from '@cozeloop/components';
import { ToolType } from '@cozeloop/api-schema/prompt';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import {
  Button,
  Col,
  Modal,
  Row,
  Select,
  Space,
  Toast,
  Typography,
} from '@coze-arch/coze-design';

import { type ToolModalProps } from './type';

const TEMPLATE_DATA = `{
  "name": "get_weather",
  "description": "Determine weather in my location",
  "parameters": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "The city and state e.g. San Francisco, CA"
      },
      "unit": {
        "type": "string",
        "enum": [
          "c",
          "f"
        ]
      }
    },
    "required": [
      "location"
    ]
  }
}`;

interface ToolSchemaProps {
  name?: string;
  description?: string;
  parameters?: any;
}
export function ToolModal({
  visible,
  disabled,
  data,
  onClose,
  onConfirm,
  tools,
}: ToolModalProps) {
  const [mockType, setMockType] = useState('text');

  const toolSchema = useMemo(() => {
    if (data?.function) {
      const toolObj: ToolSchemaProps = {
        name: data.function.name,
        description: data.function.description,
      };
      if (data.function.parameters) {
        toolObj.parameters = safeJsonParse(data.function.parameters);
      }
      return JSON.stringify(toolObj, null, 2);
    }

    return '';
  }, [JSON.stringify(data || {})]);

  const [schema, setSchema] = useState<string>();
  const [mockValue, setMockValue] = useState<string>();
  const isCreate = !data;

  const canSaveTool = useMemo(() => {
    if (schema) {
      const schemaObj = safeJsonParse<ToolSchemaProps>(schema);
      if (
        !schemaObj?.name ||
        !/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(schemaObj.name)
      ) {
        return false;
      }
      return true;
    }
  }, [schema, JSON.stringify(tools), isCreate]);

  const handleSaveTool = () => {
    if (disabled) {
      onClose?.();
    }
    if (!schema) {
      return;
    }

    const schemaObj = safeJsonParse<ToolSchemaProps>(schema);
    const toolObj = {
      name: schemaObj?.name,
      description: schemaObj?.description,
      parameters: schemaObj?.parameters
        ? JSON.stringify(schemaObj?.parameters)
        : '',
    };
    const tool = {
      type: ToolType.Function,
      function: toolObj,
      mock_response: mockValue,
    };

    const hasItem =
      tools?.find(it => it?.function?.name === toolObj?.name) &&
      data?.function?.name !== toolObj?.name;

    if (hasItem) {
      Toast.warning({
        content: I18n.t('method_exists'),
        zIndex: 99999,
      });
      return;
    }
    onConfirm?.(tool, !isCreate, data);
  };

  useEffect(() => {
    setSchema(toolSchema);
  }, [toolSchema]);

  useEffect(() => {
    setMockValue(data?.mock_response);
  }, [data?.mock_response]);

  useEffect(() => {
    if (!visible) {
      setSchema(undefined);
      setMockValue(undefined);
    }
  }, [visible]);

  return (
    <Modal
      data-btm="c90583"
      title={data?.function?.name || I18n.t('new_function')}
      width={960}
      visible={visible}
      onCancel={onClose}
      okButtonProps={{ disabled: !canSaveTool }}
      maskClosable={false}
      footer={
        disabled ? null : (
          <Space>
            <Button className="mr-2" onClick={onClose} color="primary">
              {I18n.t('cancel')}
            </Button>
            <TooltipWhenDisabled
              content={I18n.t('method_name_rule')}
              disabled={Boolean(schema && !canSaveTool)}
            >
              <Button
                onClick={handleSaveTool}
                disabled={!canSaveTool}
                data-btm="d84667"
                data-btm-title={I18n.t('global_btn_confirm')}
              >
                {I18n.t('global_btn_confirm')}
              </Button>
            </TooltipWhenDisabled>
          </Space>
        )
      }
      hasScroll={false}
    >
      <Row gutter={16}>
        <Col span={14}>
          <div className="flex justify-between items-center w-full h-8 mb-2">
            <Typography.Text
              className="font-semibold flex items-center"
              type="tertiary"
            >
              SCHEMA
              <IconCozCopy
                className="ml-2 hover:text-semi-primary cursor-pointer"
                onClick={() => handleCopy(schema || '')}
              />
            </Typography.Text>
            {disabled ? null : (
              <Button
                size="small"
                onClick={() => {
                  setSchema(TEMPLATE_DATA);
                  setMockType('text');
                  setMockValue('Sunny');
                }}
                data-btm="d62543"
                data-btm-title={I18n.t('insert_template')}
              >
                {I18n.t('insert_template')}
              </Button>
            )}
          </div>
          <SchemaEditor
            language="json"
            value={schema}
            onChange={v => setSchema(v)}
            showLineNumbs
            readOnly={disabled}
          />
        </Col>
        <Col span={10}>
          <div className="flex justify-between items-center w-full h-8 mb-2">
            <Typography.Text className="font-semibold" type="tertiary">
              {I18n.t('default_mock_value')}
            </Typography.Text>
            <Select
              value={mockType}
              onChange={v => setMockType(v as string)}
              size="small"
              zIndex={2001}
            >
              <Select.Option value="text">TEXT</Select.Option>
              <Select.Option value="json">JSON</Select.Option>
            </Select>
          </div>
          <SchemaEditor
            language={mockType}
            value={mockValue}
            onChange={v => setMockValue(v)}
            placeholder={I18n.t('input_mock_value_here')}
            readOnly={disabled}
          />
        </Col>
      </Row>
    </Modal>
  );
}
