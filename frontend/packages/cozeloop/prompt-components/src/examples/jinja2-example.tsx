// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState } from 'react';
import { Card, Tabs, Space, Typography, Alert } from 'antd';
import {
  PromptBasicEditor,
  TemplatePreview,
} from '../index';

const { Title, Paragraph } = Typography;

export function Jinja2Example() {
  const [template, setTemplate] = useState(`
Hello {{ name }}!

{% if weather %}
The weather today is {{ weather }}.
{% endif %}

{% for item in items %}
- {{ item|upper }}
{% endfor %}

Current time: {{ now()|strftime('%Y-%m-%d %H:%M:%S') }}

{% if temperature > 25 %}
It's hot today! Temperature: {{ temperature }}°C
{% elif temperature > 15 %}
It's warm today! Temperature: {{ temperature }}°C
{% else %}
It's cold today! Temperature: {{ temperature }}°C
{% endif %}

{% set message = "Welcome to Jinja2!" %}
{{ message|upper|truncate(20) }}
  `.trim());

  const [variables, setVariables] = useState({
    name: 'World',
    weather: 'sunny',
    items: 'apple,banana,cherry',
    temperature: '28',
  });

  const [validationResult, setValidationResult] = useState<{
    isValid: boolean;
    error?: string;
  }>({ isValid: true });

  const handleTemplateChange = (value: string) => {
    setTemplate(value);
  };

  const handleTemplateValidation = (isValid: boolean, error?: string) => {
    setValidationResult({ isValid, error });
  };

  const handlePreviewResult = (result: string) => {
    console.log('Template preview result:', result);
  };

  return (
    <div style={{ padding: 24, maxWidth: 1200, margin: '0 auto' }}>
      <Title level={2}>Jinja2 Template System Demo</Title>

      <Alert
        message="Jinja2 Template Features"
        description="This demo showcases the new Jinja2 template engine with syntax highlighting, auto-completion, validation, and real-time preview."
        type="info"
        style={{ marginBottom: 24 }}
      />

      <Tabs
        defaultActiveKey="editor"
        items={[
          {
            key: 'editor',
            label: 'Template Editor',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Card title="Jinja2 Template Editor" size="small">
                  <Paragraph>
                    The editor now supports Jinja2 syntax with:
                  </Paragraph>
                  <ul>
                    <li>Syntax highlighting for Jinja2 tags, variables, and filters</li>
                    <li>Auto-completion for keywords, filters, and functions</li>
                    <li>Real-time syntax validation</li>
                    <li>Variable management and template preview</li>
                  </ul>
                </Card>

                <PromptBasicEditor
                  defaultValue={template}
                  templateType="jinja2"
                  enableJinja2Preview
                  height={300}
                  onChange={handleTemplateChange}
                  onTemplateValidation={handleTemplateValidation}
                  variables={Object.entries(variables).map(([key, value]) => ({
                    key,
                    desc: `Variable: ${key}`,
                    type: 'string' as const,
                  }))}
                />

                {!validationResult.isValid && (
                  <Alert
                    type="error"
                    message="Template Validation Error"
                    description={validationResult.error}
                    style={{ marginTop: 16 }}
                  />
                )}

                {validationResult.isValid && (
                  <Alert
                    type="success"
                    message="Template is valid"
                    description="Your Jinja2 template syntax is correct!"
                    style={{ marginTop: 16 }}
                  />
                )}
              </Space>
            ),
          },
          {
            key: 'preview',
            label: 'Template Preview',
            children: (
              <TemplatePreview
                template={template}
                templateType="jinja2"
                variables={variables}
                onPreview={handlePreviewResult}
              />
            ),
          },
          {
            key: 'features',
            label: 'Features Overview',
            children: (
              <Card title="Jinja2 Features Overview">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Card title="Syntax Highlighting" size="small">
                    <ul>
                      <li><strong>Variables:</strong> <code>{'{{ variable }}'}</code> - Highlighted in orange</li>
                      <li><strong>Statements:</strong> <code>{'{% if condition %}'}</code> - Highlighted in blue</li>
                      <li><strong>Filters:</strong> <code>{'{{ text|upper }}'}</code> - Highlighted in purple</li>
                      <li><strong>Comments:</strong> <code>{'{# comment #}'}</code> - Highlighted in gray</li>
                    </ul>
                  </Card>

                  <Card title="Auto-completion" size="small">
                    <ul>
                      <li><strong>Keywords:</strong> if, for, set, block, macro, etc.</li>
                      <li><strong>Filters:</strong> upper, lower, trim, default, length, etc.</li>
                      <li><strong>Functions:</strong> range, dict, now, etc.</li>
                      <li><strong>Smart Context:</strong> Only shows relevant suggestions in Jinja2 blocks</li>
                    </ul>
                  </Card>

                  <Card title="Syntax Validation" size="small">
                    <ul>
                      <li><strong>Bracket Matching:</strong> Checks {{ }} and {% %} pairs</li>
                      <li><strong>Control Structure:</strong> Validates if/endif, for/endfor pairs</li>
                      <li><strong>Filter Validation:</strong> Ensures valid filter names</li>
                      <li><strong>Real-time Feedback:</strong> Shows errors as you type</li>
                    </ul>
                  </Card>

                  <Card title="Template Preview" size="small">
                    <ul>
                      <li><strong>Variable Management:</strong> Add, edit, and remove variables</li>
                      <li><strong>Real-time Rendering:</strong> See results as you type</li>
                      <li><strong>Error Handling:</strong> Clear error messages for invalid templates</li>
                      <li><strong>Responsive Layout:</strong> Side-by-side variable editing and preview</li>
                    </ul>
                  </Card>
                </Space>
              </Card>
            ),
          },
        ]}
      />
    </div>
  );
}
