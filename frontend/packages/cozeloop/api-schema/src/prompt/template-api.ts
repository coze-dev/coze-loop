// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

const apiClient = {
  async request<T>(url: string, options: RequestInit): Promise<T> {
    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return response.json();
  }
};

export interface ValidateTemplateRequest {
  template: string;
  template_type: string;
}

export interface ValidateTemplateResponse {
  code: number;
  msg: string;
  data: {
    is_valid: boolean;
    error_message?: string;
  };
}

export interface PreviewTemplateRequest {
  template: string;
  template_type: string;
  variables: Record<string, string>;
}

export interface PreviewTemplateResponse {
  code: number;
  msg: string;
  data: {
    result: string;
  };
}

export const TemplateApi = {
  /**
   * 验证Jinja2模板语法
   * @param data 验证请求参数
   * @returns 验证结果
   */
  validateTemplate: (data: ValidateTemplateRequest) =>
    apiClient.request<ValidateTemplateResponse>('/v1/loop/prompts/validate-template', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /**
   * 预览Jinja2模板渲染结果
   * @param data 预览请求参数
   * @returns 预览结果
   */
  previewTemplate: (data: PreviewTemplateRequest) =>
    apiClient.request<PreviewTemplateResponse>('/v1/loop/prompts/preview-template', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};
