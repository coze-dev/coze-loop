import { useLocale } from './context';

// Example component demonstrating how to use the useLocale hook
export const I18nExampleComponent = () => {
  const { t, locale } = useLocale();

  return (
    <div>
      <h1>{t('analytics_trace_description')}</h1>
      <p>{t('analytics_trace_runtime', { count: 1 })}</p>
      <p>{t('analytics_trace_runtime', { count: 5 })}</p>
      <p>{t('price', { currency: 'USD', value: 123.45 })}</p>
      <p>Current locale: {locale}</p>
    </div>
  );
};

export { useLocale };
