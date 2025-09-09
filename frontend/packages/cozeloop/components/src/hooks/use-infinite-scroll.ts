// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
import { useMemo, useState } from 'react';

import { getTargetElement } from 'ahooks/lib/utils/domTarget';
import {
  type InfiniteScrollOptions,
  type Data,
  type Service,
} from 'ahooks/lib/useInfiniteScroll/types';
import {
  useEventListener,
  useMemoizedFn,
  useRequest,
  useUpdateEffect,
} from 'ahooks';

import { getClientHeight, getScrollHeight, getScrollTop } from '../utils/rect';

/**
 * ahook的实现，在刷新列表时会出现以下两个问题
 * 1. 发送存量数据对应的列表请求
 * 2. 列表请求重复
 * 因此fork ahook 实现，并将reload含义定义为刷新列表，并回到第一页
 * @param service
 * @param options
 * @returns
 */
export const useInfiniteScroll = <TData extends Data>(
  service: Service<TData>,
  options: InfiniteScrollOptions<TData> = {},
) => {
  const {
    target,
    isNoMore,
    threshold = 100,
    reloadDeps = [],
    manual,
    onBefore,
    onSuccess,
    onError,
    onFinally,
  } = options;

  const [finalData, setFinalData] = useState<TData>();
  const [loadingMore, setLoadingMore] = useState(false);

  const noMore = useMemo(() => {
    if (!isNoMore) {
      return false;
    }
    return isNoMore(finalData);
  }, [finalData]);

  const { loading, error, run, runAsync, cancel } = useRequest(
    async (lastData?: TData) => {
      const currentData = await service(lastData);
      if (!lastData) {
        setFinalData({
          ...currentData,
          list: [...(currentData.list ?? [])],
        });
      } else {
        setFinalData({
          ...currentData,
          list: [...(lastData.list ?? []), ...currentData.list],
        });
      }
      return currentData;
    },
    {
      manual,
      onFinally: (_, d, e) => {
        setLoadingMore(false);
        onFinally?.(d, e);
      },
      onBefore: () => onBefore?.(),
      onSuccess: d => {
        // setTimeout(() => {
        //   scrollMethod();
        // });
        onSuccess?.(d);
        setTimeout(() => {
          checkFirstScreen();
        });
      },
      onError,
    },
  );

  const loadMore = useMemoizedFn(() => {
    if (noMore) {
      return;
    }
    setLoadingMore(true);
    run(finalData);
  });

  const loadMoreAsync = useMemoizedFn(() => {
    if (noMore) {
      // eslint-disable-next-line prefer-promise-reject-errors
      return Promise.reject();
    }
    setLoadingMore(true);
    return runAsync(finalData);
  });

  const reload = () => {
    setLoadingMore(false);
    setFinalData(undefined);
    return run();
  };

  const reloadAsync = () => {
    setLoadingMore(false);
    return runAsync();
  };

  const checkFirstScreen = () => {
    let el = getTargetElement(target);
    if (!el) {
      return;
    }

    el = el === document ? document.documentElement : el;
    const scrollHeight = getScrollHeight(el);
    const clientHeight = getClientHeight(el);
    if (scrollHeight <= clientHeight) {
      // 首屏没满，自动 loadMore
      loadMore();
    }
  };

  const scrollMethod = () => {
    let el = getTargetElement(target);
    if (!el) {
      return;
    }

    el = el === document ? document.documentElement : el;

    const scrollTop = getScrollTop(el);
    const scrollHeight = getScrollHeight(el);
    const clientHeight = getClientHeight(el);

    if (scrollHeight - scrollTop <= clientHeight + threshold) {
      loadMore();
    }
  };

  useEventListener(
    'scroll',
    () => {
      if (loading || loadingMore) {
        return;
      }
      scrollMethod();
    },
    { target },
  );

  useUpdateEffect(() => {
    setFinalData(undefined);
    run();
  }, [...reloadDeps]);

  return {
    data: finalData,
    loading: !loadingMore && loading,
    error,
    loadingMore,
    noMore,
    loadMore,
    loadMoreAsync,
    reload: useMemoizedFn(reload),
    reloadAsync: useMemoizedFn(reloadAsync),
    mutate: setFinalData,
    cancel,
  };
};
