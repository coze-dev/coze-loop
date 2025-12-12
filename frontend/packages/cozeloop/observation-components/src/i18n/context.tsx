// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';

import IntlMessageFormat from 'intl-messageformat';

import { i18nService } from './service';

// Define types
export interface Locale {
  language: string;
  locale: Record<string, string>;
}

export interface I18nContextType {
  t: (key: string, options?: Record<string, unknown>) => string;
  locale?: string;
}

// Create context
const I18nContext = createContext<I18nContextType>({
  t: (key: string) => key,
  locale: undefined,
});

// Create provider
export interface I18nProviderProps {
  defaultLocale?: Locale;
  children: React.ReactNode;
}

export const I18nProvider: React.FC<I18nProviderProps> = ({
  defaultLocale,
  children,
}) => {
  const isInitialized = useRef(false);
  const [initialized, setInitialized] = useState(false);
  const t = useCallback(
    (key: string, options?: Record<string, unknown>): string => {
      if (!key) {
        return '';
      }

      // Handle all formatting using intl-messageformat
      if (options && Object.keys(options).length > 0) {
        let translation = key;
        if (defaultLocale?.locale && key in defaultLocale.locale) {
          translation = String(defaultLocale.locale[key]);
        }

        // Use intl-messageformat for all formatting needs
        try {
          const formatter = new IntlMessageFormat(
            translation,
            defaultLocale?.language,
          );
          return formatter.format(options) as string;
        } catch (error) {
          console.error('Error formatting message:', error);
          // Fallback for simple cases
          if (options?.currency) {
            // Simple currency formatting fallback
            return `${options.currency} ${(Number(options.value) || 0).toFixed(2)}`;
          }

          // Simple interpolation fallback
          Object.keys(options).forEach(optionKey => {
            if (
              optionKey !== 'currency' &&
              optionKey !== 'value' &&
              optionKey !== 'count'
            ) {
              translation = translation.replace(
                new RegExp(`{{${optionKey}}}`, 'g'),
                String(options[optionKey]),
              );
            }
          });
          return translation;
        }
      }

      // Default translation
      if (defaultLocale?.locale && key in defaultLocale.locale) {
        return String(defaultLocale.locale[key]);
      }

      return key;
    },
    [defaultLocale],
  );

  useEffect(() => {
    if (defaultLocale && !isInitialized.current) {
      i18nService.init(defaultLocale);
      isInitialized.current = true;
      setInitialized(true);
    }
  }, [defaultLocale]);

  const contextValue = {
    t,
    locale: defaultLocale?.language,
  };

  if (!initialized) {
    return null;
  }

  return (
    <I18nContext.Provider value={contextValue}>{children}</I18nContext.Provider>
  );
};

// Create useLocale hook
export const useLocale = () => {
  const context = useContext(I18nContext);
  return context;
};
