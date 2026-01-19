# I18n Implementation for Observation Components

This directory contains the internationalization (i18n) implementation for the observation components.

## Features

1. **Translation**: Basic string translation using language resource files
2. **Currency Formatting**: Support for formatting currency values based on locale
3. **Pluralization**: Support for plural forms of strings based on count values
4. **Interpolation**: Support for dynamic value interpolation in strings

## Usage

### 1. Setting up the Provider

In your application's root component, wrap your app with the `ConfigProvider` and provide the locale data:

```tsx
import { ConfigProvider } from './config-provider';
import enUS from './i18n/resource/en-US.json';
import zhCN from './i18n/resource/zh-CH.json';

const localeData = {
  'en-US': enUS,
  'zh-CH': zhCN,
};

const App = () => {
  const locale = {
    locale: 'en-US',
    messages: localeData['en-US'],
  };
  
  return (
    <ConfigProvider i18nLocale={locale}>
      {/* Your app components */}
    </ConfigProvider>
  );
};
```

### 2. Using the useLocale Hook

In your components, use the `useLocale` hook to access the translation function:

```tsx
import { useLocale } from './i18n/context';

const MyComponent = () => {
  const { t } = useLocale();
  
  return (
    <div>
      <h1>{t('analytics_trace_description')}</h1>
      <p>{t('analytics_trace_runtime', { count: 5 })}</p>
      <p>{t('price', { currency: 'USD', value: 123.45 })}</p>
    </div>
  );
};
```

## Features Details

### Translation

Basic translation using the `t` function:

```tsx
const { t } = useLocale();
t('key_name');
```

### Currency Formatting

Format currency values with the locale-appropriate format:

```tsx
const { t } = useLocale();
t('price', { currency: 'USD', value: 123.45 });
// Output: $123.45 (for en-US locale)
```

### Pluralization

Handle plural forms automatically based on count:

```tsx
const { t } = useLocale();
t('item_count', { count: 1 });
// Will look for 'item_count_one' key first, then fall back to 'item_count'

t('item_count', { count: 5 });
// Will look for 'item_count_other' key first, then fall back to 'item_count'
```

### Interpolation

Interpolate dynamic values into strings using double curly braces:

```tsx
const { t } = useLocale();
t('greeting', { name: 'John' });
// Will look for 'greeting' key and replace {{name}} with 'John'
```

In your resource files, define the string with placeholders:

```json
{
  "greeting": "Hello {{name}}, how are you today?"
}
```

## Resource Files

The locale resource files are stored in the `resource` directory:

- `en-US.json`: English (United States) translations
- `zh-CH.json`: Chinese (Simplified) translations

These files contain key-value pairs where the key is the translation key and the value is the translated string.

## Implementation Details

The implementation uses:

1. React Context for providing the translation function throughout the component tree
2. `intl-messageformat` library for all formatting needs (translation, currency formatting, pluralization, and interpolation)
3. Custom logic for handling translation fallbacks and plural forms