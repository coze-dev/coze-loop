// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
/* eslint-disable @typescript-eslint/naming-convention */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useSearchParams } from 'react-router-dom';
import React, {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';

import { omit } from 'lodash-es';
import { safeJsonParse } from '@cozeloop/toolkit';
import { type Form } from '@coze-arch/coze-design';

export type SearchFormFilterRecord = Record<string, any>;

export interface SearchFormRef {
  getValues: () => SearchFormFilterRecord | undefined;
}

interface SearchFormProps {
  className?: string;
  children: any;
  SemiForm: typeof Form;
  onSearch?: (v: SearchFormFilterRecord) => Promise<any>;
  initValue?: SearchFormFilterRecord;
}

/**
 * 如果对象的value为对象，将 value 转换为字符串
 * @param value
 * @returns
 */
const stringifyObjectValue = (value: Record<string, any>) =>
  Object.keys(value).reduce(
    (acc, key) => {
      const val = value[key];
      if (typeof val === 'object') {
        acc[key] = JSON.stringify(val);
      } else {
        acc[key] = val;
      }
      return acc;
    },
    {} as unknown as Record<string, string>,
  );

const parseObjectValue = (data: Record<string, string>) =>
  Object.keys(data).reduce(
    (acc, key) => {
      const value = data[key];
      if (value.startsWith('{') || value.startsWith('[')) {
        acc[key] = safeJsonParse(value) || value;
      } else {
        acc[key] = value;
      }
      return acc;
    },
    {} as unknown as Record<string, any>,
  );

export const SearchForm = forwardRef<SearchFormRef, SearchFormProps>(
  ({ className, children, onSearch, SemiForm, initValue = {} }, ref) => {
    const formRef = useRef<Form<SearchFormFilterRecord>>(null);

    const [initValueAndParams] = useState(() => {
      const params = new URLSearchParams(window.location.search);

      const initParams: Record<string, string> = {};
      params.forEach((value, key) => {
        initParams[key] = value;
      });

      const urlValues = parseObjectValue(initParams);
      const defaultValue = { ...initValue, ...urlValues };
      const defaultSearchParams = new URLSearchParams(
        stringifyObjectValue(defaultValue),
      );
      const filterValue = omit(defaultValue, ['page', 'pageSize']);
      if (Object.keys(filterValue).length !== 0) {
        onSearch?.(filterValue);
      }
      return {
        defaultValue: filterValue,
        defaultSearchParams,
      };
    });
    const [searchParams, setSearchParams] = useSearchParams();

    const formValueChange = useCallback(
      (allValues: SearchFormFilterRecord) => {
        const searchResult = onSearch?.(allValues);
        searchResult
          ?.then(() => {
            setSearchParams(
              prev => {
                const v = stringifyObjectValue(allValues || {});
                prev.forEach((_value, key) => {
                  if (!['page', 'pageSize', 'tab'].includes(key)) {
                    prev.delete(key);
                  }
                });
                Object.keys(v).forEach(key => {
                  prev.set(key, v[key]);
                });
                return prev;
              },
              { replace: true },
            );
          })
          .catch(e => console.error(e));
      },
      [searchParams],
    );

    useImperativeHandle(ref, () => ({
      getValues: () => formRef.current?.formApi?.getValues?.(),
    }));

    type ChangeValue = typeof initValueAndParams.defaultValue;
    return children ? (
      <SemiForm<ChangeValue>
        ref={formRef}
        initValues={initValueAndParams.defaultValue}
        onValueChange={formValueChange}
        layout="horizontal"
        className={className}
      >
        {children}
      </SemiForm>
    ) : null;
  },
);
