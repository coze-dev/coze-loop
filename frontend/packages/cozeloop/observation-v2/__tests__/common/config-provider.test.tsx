import React from 'react';

import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import '@testing-library/jest-dom';

import { ConfigProvider } from '../../src/config-provider';

describe('ConfigProvider', () => {
  it('should render container element correctly', () => {
    render(
      <ConfigProvider>
        <div>Test Child</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element when no props are passed', () => {
    // 只检查容器元素是否存在
    render(
      <ConfigProvider>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element with theme prop', () => {
    const themeConfig = { '--primary-color': '#1890ff', '--font-size': '14px' };

    // 只检查容器元素是否存在
    render(
      <ConfigProvider theme={themeConfig}>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element with timeZone prop', () => {
    const timeZone = 'Asia/Shanghai';

    // 只检查容器元素是否存在
    render(
      <ConfigProvider timeZone={timeZone}>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element with locale prop', () => {
    const locale = {
      language: 'zh-CN',
      locale: { 'test.key': '测试值' },
    };

    // 只检查容器元素是否存在
    render(
      <ConfigProvider locale={locale}>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element with all props', () => {
    const themeConfig = { '--primary-color': '#1890ff' };
    const timeZone = 'Asia/Shanghai';
    const locale = {
      language: 'zh-CN',
      locale: { 'test.key': '测试值' },
    };

    // 只检查容器元素是否存在
    render(
      <ConfigProvider theme={themeConfig} timeZone={timeZone} locale={locale}>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render container element with sendEvent prop', () => {
    const sendEventMock = vi.fn();

    // 只检查容器元素是否存在
    render(
      <ConfigProvider sendEvent={sendEventMock}>
        <div>Test</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
  });

  it('should render children inside correct container div', () => {
    render(
      <ConfigProvider>
        <div>Test Content</div>
      </ConfigProvider>,
    );

    const container = document.getElementById(
      'cozeloop-observation-components',
    );
    expect(container).toBeInTheDocument();
    expect(container).toHaveClass(
      'w-full',
      'h-full',
      'max-w-full',
      'overflow-hidden',
    );
  });
});
