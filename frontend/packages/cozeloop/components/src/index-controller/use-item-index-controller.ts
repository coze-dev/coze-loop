// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/no-destructuring-use-request */
import { useState, useCallback, useEffect, useRef } from 'react';

import { useLatest, useRequest } from 'ahooks';

type FetchRecordItemsFn<RecordItem> = (params: {
  pageSize: number;
  pageIndex: number;
}) => Promise<{ list: RecordItem[]; total: number }>;

/**
 * 索引控制器参数
 */
export interface IndexControllerOptions<RecordItem = Record<string, unknown>> {
  /** 默认选中的记录索引 */
  defaultIndex: number;
  /** 刷新记录列表 */
  fetchRecordItems: FetchRecordItemsFn<RecordItem>;
  /** 批量获取记录数据的个数 */
  batchSize?: number;
  /** 获取记录详情 */
  fetchRecordItem: (record: RecordItem) => Promise<RecordItem | undefined>;
  /** 切换记录的回调 */
  onRecordChange?: (record: RecordItem) => void;
}

/**
 * 索引控制器状态&API
 */
export interface IndexControllerStore {
  /** 是否有上一条 */
  hasPrevious: boolean;
  /** 是否有下一条 */
  hasNext: boolean;
  /** 当前记录在列表中的位置信息 */
  currentIndex: number;
  /** 总记录数 */
  total: number;
  /** 切换到上一条 */
  goToPrevious: () => void;
  /** 切换到下一条 */
  goToNext: () => void;
  /** 加载状态 */
  loading: boolean;
}

async function batchFetchRecordItems<RecordItem>({
  currentIndex = 0,
  batchSize,
  total,
  fetchRecordItems,
}: {
  currentIndex: number;
  total?: number;
  batchSize: number;
  fetchRecordItems: FetchRecordItemsFn<RecordItem>;
}) {
  const pageIndex = Math.floor(currentIndex / batchSize) + 1;
  // index所在区间的前一个区间
  const prevPromise =
    pageIndex > 1
      ? fetchRecordItems({
          pageSize: batchSize,
          pageIndex: pageIndex - 1,
        })
      : Promise.resolve({ list: [], total: 0 });

  const nextPromise =
    typeof total !== 'number' || currentIndex + batchSize < total
      ? fetchRecordItems({
          pageSize: batchSize,
          pageIndex: pageIndex + 1,
        })
      : Promise.resolve({ list: [], total: 0 });
  const res = await Promise.all([
    // index所在区间的前一个区间
    prevPromise,
    // index所在区间
    fetchRecordItems({
      pageSize: batchSize,
      pageIndex,
    }),
    // index所在区间的后一个区间
    nextPromise,
  ]);
  const allList = res.flatMap(item => item.list || []);
  // 计算当前index所在区间的前一个区间的起始index
  const prevStartIndex = Math.max(
    0,
    currentIndex - (currentIndex % batchSize) - batchSize,
  );
  const allItems: (RecordItem | undefined)[] = [];
  const list = allList || [];
  for (let i = 0; i < list.length; i++) {
    allItems[prevStartIndex + i] = list[i];
  }
  return { list: allItems, total: res?.[1]?.total || 0 };
}

/**
 * 索引控制器
 * 用于在详情抽屉中实现上一条、下一条功能
 */
export function useItemIndexController<RecordItem = Record<string, unknown>>({
  defaultIndex,
  onRecordChange,
  fetchRecordItem,
  fetchRecordItems,
  batchSize = 50,
}: IndexControllerOptions<RecordItem>): IndexControllerStore {
  const [allRecords, setAllRecords] = useState<(RecordItem | undefined)[]>([]);
  const [currentIndex, setCurrentIndex] = useState(defaultIndex);
  const [total, setTotalCount] = useState(0);
  const totalRef = useLatest(total);
  // 上一次触发请求的index
  const lastCurrentIndexRef = useRef<number>(-1);

  // 列表请求，根据 currentIndex 所在位置的前后各自获取batchSize个数量
  const { loading: listLoading, run: updateList } = useRequest(async () => {
    const res = await batchFetchRecordItems({
      currentIndex,
      total: total || undefined,
      batchSize,
      fetchRecordItems,
    });
    setTotalCount(res.total);
    setAllRecords(res.list);
    lastCurrentIndexRef.current = currentIndex;
  });

  // 详情请求
  const { runAsync: fetchRecordItemAsync, loading: itemLoading } = useRequest(
    async (record: RecordItem) => {
      const res = await fetchRecordItem(record);
      return res;
    },
    { manual: true },
  );

  // 切换到上一条记录
  const goToPrevious = useCallback(async () => {
    try {
      const record = allRecords[currentIndex - 1];
      if (currentIndex > 0 && record) {
        const previousRecord = await fetchRecordItemAsync(record);
        if (previousRecord) {
          onRecordChange?.(previousRecord);
          setCurrentIndex(currentIndex - 1);
        }
      }
    } catch (e) {
      console.warn(e);
    }
  }, [currentIndex, fetchRecordItemAsync, allRecords, onRecordChange]);

  // 切换到下一条记录
  const goToNext = useCallback(async () => {
    try {
      const record = allRecords[currentIndex + 1];
      if (currentIndex < totalRef.current - 1 && record) {
        const nextRecord = await fetchRecordItemAsync(record);
        if (nextRecord) {
          onRecordChange?.(nextRecord);
          setCurrentIndex(currentIndex + 1);
        }
      }
    } catch (e) {
      console.warn(e);
    }
  }, [
    currentIndex,
    totalRef,
    fetchRecordItemAsync,
    allRecords,
    onRecordChange,
  ]);

  useEffect(() => {
    // fetchRecordItems 会根据 currentIndex 是否超过上次触发请求的list范围来判断是否需要触发请求
    if (
      lastCurrentIndexRef.current !== -1 &&
      Math.abs(currentIndex - lastCurrentIndexRef.current) >= batchSize
    ) {
      updateList();
    }
  }, [currentIndex]);

  return {
    hasPrevious: currentIndex > 0,
    hasNext: currentIndex < total - 1,
    currentIndex,
    total,
    goToPrevious,
    goToNext,
    loading: listLoading || itemLoading,
  };
}
