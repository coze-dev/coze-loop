import {
  type TraceContextType,
  useTraceContext,
} from '@/features/trace-list/contexts/trace-context';

export function useTraceStore(
  selector?: (state: TraceContextType) => TraceContextType,
) {
  const context = useTraceContext();
  if (!selector) {
    return context;
  }
  return selector(context);
}
