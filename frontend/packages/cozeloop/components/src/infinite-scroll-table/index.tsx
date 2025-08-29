import {
  type ForwardedRef,
  forwardRef,
  useRef,
  useImperativeHandle,
} from 'react';

import classNames from 'classnames';
import {
  type Data,
  type InfiniteScrollOptions,
  type Service,
} from 'ahooks/lib/useInfiniteScroll/types';
import { useSize, useInfiniteScroll } from 'ahooks';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { EmptyState, type TableProps } from '@coze-arch/coze-design';

import { LoopTable } from '@/table';

interface ExpandData extends Data {
  hasMore?: boolean;
}
interface InfiniteScrollTableProps<TData extends ExpandData> {
  service: Service<TData>;
  options?: InfiniteScrollOptions<TData>;
}

export interface InfiniteScrollTableRef {
  hookRes: ReturnType<typeof useInfiniteScroll>;
}

// 定义组件的 Props 类型
type Props<TData extends ExpandData> = TableProps &
  InfiniteScrollTableProps<TData>;

// 显式指定 forwardRef 的类型
export const InfiniteScrollTable: <TData extends ExpandData>(
  props: Props<TData> & { ref?: ForwardedRef<InfiniteScrollTableRef> },
) => React.ReactElement | null = forwardRef(
  <TData extends ExpandData>(
    { service, options, className, ...restTableProps }: Props<TData>,
    ref: ForwardedRef<InfiniteScrollTableRef>,
  ): JSX.Element => {
    const containerRef = useRef<HTMLDivElement>(null);

    const hookRes = useInfiniteScroll(d => service?.(d), {
      isNoMore: d => !d?.hasMore,
      ...options,
      reloadDeps: [...(options?.reloadDeps || [])],
    });

    const { data, loading, loadingMore, noMore, loadMore } = hookRes;

    const scrollSize = useSize(containerRef);
    const height = scrollSize?.height || 0;

    useImperativeHandle(ref, () => ({
      // @ts-expect-error type
      hookRes,
    }));

    return (
      <div
        className={classNames('w-full h-full overflow-hidden', className)}
        ref={containerRef}
      >
        <LoopTable
          tableProps={{
            ...restTableProps.tableProps,
            loading: loading || loadingMore,
            pagination: false,
            dataSource: data?.list || [],
            scroll: {
              y: height - 48,
              ...restTableProps.tableProps?.scroll,
            },
          }}
          empty={
            restTableProps.empty ?? (
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusEmpty />}
                title="暂无数据"
              />
            )
          }
          enableLoad={true}
          loadMode="cursor"
          hasMore={!noMore}
          onLoad={loadMore}
          offsetY={0}
          strictDataSourceProp
        />
      </div>
    );
  },
) as <TData extends ExpandData>(
  props: Props<TData> & { ref?: ForwardedRef<InfiniteScrollTableRef> },
) => React.ReactElement | null;
