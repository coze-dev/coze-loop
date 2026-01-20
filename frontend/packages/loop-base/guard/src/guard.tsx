// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  cloneElement,
  type ReactElement,
  type PropsWithChildren,
  useState,
  useRef,
} from 'react';

import { type GuardPoint, GuardActionType } from './types';
import { useGuard } from './hooks/use-guard';

interface Props {
  point: GuardPoint;
  realtime?: boolean;
  // 触发拦截时响应
  onGuard?: () => void;
  // 某些情况下，不触发拦截
  ignore?: boolean;
}

export function Guard({
  point,
  realtime,
  onGuard,
  children,
  ignore,
}: PropsWithChildren<Props>) {
  const guard = useGuard({ point });

  const [loading, setLoading] = useState(false);
  const childrenNode = children as ReactElement;

  const ref = useRef(guard);
  ref.current = guard;

  const handleClick = async (e: unknown) => {
    let originalClick = childrenNode.props.onClick;
    originalClick =
      typeof originalClick === 'function'
        ? originalClick
        : () => {
            /* empty*/
          };

    if (realtime) {
      try {
        setLoading(true);
        await guard.updateData();
      } catch (error) {
        console.log(error);
      } finally {
        setLoading(false);
      }
    }
    setTimeout(() => {
      if (ref.current.data.type === GuardActionType.ACTION) {
        originalClick(e);
      } else if (ref.current.data.type === GuardActionType.GUARD) {
        ref.current.data.preprocess(() => originalClick(e));
        onGuard?.();
      }
    }, 0);
  };
  if (ignore) {
    return children;
  }
  if (!childrenNode || guard.data.hidden) {
    return null;
  }

  return cloneElement(childrenNode, {
    disabled: childrenNode.props.disabled || guard.data.readonly,
    loading: childrenNode.props.loading || loading,
    onClick: handleClick,
  });
}
