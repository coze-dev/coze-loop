// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { createContext, useContext } from 'react';

import { type FieldMeta } from '@cozeloop/api-schema/observation';

import { metaInfo } from '@/features/trace-list/constants/meta-info';

import { type TraceContextType, type TraceProviderProps } from './trace-types';
import { useTraceActions } from './trace-hooks';

export {
  type TraceContextState,
  type TraceContextActions,
  type TraceContextType,
} from './trace-types';

const TraceContext = createContext<TraceContextType | undefined>(undefined);

export const TraceProvider: React.FC<TraceProviderProps> = ({
  children,
  getFieldMetas,
  customViewConfig,
  customParams,
  disableEffect,
}) => {
  const actions = useTraceActions();
  const contextValue = {
    getFieldMetas: getFieldMetas
      ? getFieldMetas
      : () => {
          const newFieldMetas = { ...metaInfo.field_metas };
          return Promise.resolve(
            newFieldMetas as unknown as Record<string, FieldMeta>,
          );
        },
    customViewConfig,
    customParams,
    disableEffect,
  };
  return (
    <TraceContext.Provider
      value={{
        ...contextValue,
        setFieldMetas: actions.setFieldMetas,
        fieldMetas: actions.fieldMetas,
      }}
    >
      {children}
    </TraceContext.Provider>
  );
};

export const useTraceContext = (): TraceContextType => {
  const context = useContext(TraceContext);
  if (context === undefined) {
    throw new Error('useTraceContext must be used within a TraceProvider');
  }
  return context;
};
