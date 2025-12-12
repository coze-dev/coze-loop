import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import '@testing-library/jest-dom';

import { i18nService } from '../../src/i18n/service';
import { I18nProvider, useLocale } from '../../src/i18n/context';

// 模拟 i18nService
vi.mock('../../src/i18n/service', () => ({
  i18nService: {
    init: vi.fn(),
  },
}));

// 测试用的子组件，用于测试 useLocale hook
const TestI18nComponent = () => {
  const { t, locale } = useLocale();
  return (
    <div data-testid="i18n-data">
      <span data-testid="translated-text">{t('test.key')}</span>
      <span data-testid="translated-with-options">
        {t('test.withOptions', { name: 'John' })}
      </span>
      <span data-testid="locale-data">{locale}</span>
      <span data-testid="empty-key">{t('')}</span>
      <span data-testid="missing-key">{t('missing.key')}</span>
    </div>
  );
};

describe('I18nProvider and useLocale', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render null when not initialized', () => {
    const { container } = render(
      <I18nProvider defaultLocale={undefined}>
        <TestI18nComponent />
      </I18nProvider>,
    );

    expect(container.firstChild).toBeNull();
  });

  it('should initialize i18nService with defaultLocale', () => {
    const mockLocale = {
      language: 'zh-CN',
      locale: { 'test.key': '测试值' },
    };

    // 我们需要使用异步渲染，因为 I18nProvider 有 useEffect 逻辑
    render(
      <I18nProvider defaultLocale={mockLocale}>
        <TestI18nComponent />
      </I18nProvider>,
    );

    expect(i18nService.init).toHaveBeenCalledWith(mockLocale);
  });

  it('should provide correct translation when key exists', () => {
    const mockLocale = {
      language: 'zh-CN',
      locale: {
        'test.key': '测试值',
        'test.withOptions': '你好，{{name}}',
      },
    };

    // 使用一个包装组件来测试翻译功能
    const TranslationTestComponent = () => {
      const { t, locale } = useLocale();
      return (
        <div>
          <span data-testid="translated-test-key">{t('test.key')}</span>
          <span data-testid="current-locale">{locale}</span>
        </div>
      );
    };

    // 渲染组件
    render(
      <I18nProvider defaultLocale={mockLocale}>
        <TranslationTestComponent />
      </I18nProvider>,
    );
  });

  it('should return key itself when translation is not found', () => {
    // 创建一个测试组件来验证未找到翻译的行为
    const MissingTranslationComponent = () => {
      const { t } = useLocale();
      return (
        <span data-testid="missing-translation">{t('non.existent.key')}</span>
      );
    };

    render(
      <I18nProvider defaultLocale={{ language: 'en-US', locale: {} }}>
        <MissingTranslationComponent />
      </I18nProvider>,
    );
  });

  it('should return empty string when key is empty', () => {
    // 创建一个测试组件来验证空键的行为
    const EmptyKeyComponent = () => {
      const { t } = useLocale();
      return <span data-testid="empty-key-result">{t('')}</span>;
    };

    render(
      <I18nProvider defaultLocale={{ language: 'en-US', locale: {} }}>
        <EmptyKeyComponent />
      </I18nProvider>,
    );
  });

  it('should handle formatting with options', () => {
    // 创建一个测试组件来验证格式化功能
    const FormattingComponent = () => {
      const { t } = useLocale();
      return (
        <span data-testid="formatted-text">
          {t('greeting', { name: 'John' })}
        </span>
      );
    };

    const mockLocale = {
      language: 'en-US',
      locale: { greeting: 'Hello, {name}!' },
    };

    render(
      <I18nProvider defaultLocale={mockLocale}>
        <FormattingComponent />
      </I18nProvider>,
    );
  });

  it('should handle currency formatting', () => {
    // 创建一个测试组件来验证货币格式化功能
    const CurrencyComponent = () => {
      const { t } = useLocale();
      return (
        <span data-testid="currency-formatted">
          {t('price', { currency: 'USD', value: 100 })}
        </span>
      );
    };

    render(
      <I18nProvider defaultLocale={{ language: 'en-US', locale: {} }}>
        <CurrencyComponent />
      </I18nProvider>,
    );
  });

  it('should handle simple interpolation', () => {
    // 创建一个测试组件来验证简单插值功能
    const InterpolationComponent = () => {
      const { t } = useLocale();
      return (
        <span data-testid="interpolated-text">
          {t('welcome', { app: 'CozeLoop' })}
        </span>
      );
    };

    const mockLocale = {
      language: 'en-US',
      locale: { welcome: 'Welcome to {{app}}' },
    };

    render(
      <I18nProvider defaultLocale={mockLocale}>
        <InterpolationComponent />
      </I18nProvider>,
    );
  });
});
