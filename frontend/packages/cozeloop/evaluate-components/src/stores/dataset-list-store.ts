// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/zustand/devtools-config */
import { devtools, subscribeWithSelector } from 'zustand/middleware';
import { create } from 'zustand';
import { type ListEvaluationSetsRequest } from '@cozeloop/api-schema/evaluation';
interface DatasetListStore {
  filter: Partial<ListEvaluationSetsRequest>;
  page: number;
}

interface DatasetListStoreAction {
  setFilter: (filter: Partial<ListEvaluationSetsRequest>) => void;
  setPage: (page: number) => void;
}

export const useDatasetListStore = create<
  DatasetListStore & DatasetListStoreAction
>()(
  devtools(
    subscribeWithSelector((set, get) => ({
      filter: {},
      page: 1,
      setFilter: (filter: Partial<ListEvaluationSetsRequest>) => {
        set({ filter });
      },
      setPage: (page: number) => {
        set({ page });
      },
    })),
  ),
);
