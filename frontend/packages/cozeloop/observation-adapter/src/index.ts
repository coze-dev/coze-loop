import { type ObservationTraceAdapters } from '@cozeloop/adapter-interfaces';

import { useFetchSpanDetail } from './hooks/use-fetch-span-detail';
import { TraceDetail, TraceDetailPanel } from './components/trace-detail';
import {
  CozeloopTraceSelector,
  CozeloopTraceSelectorInitValues,
} from './components';

const getAutoEvaluateFieldMappingConfig: ObservationTraceAdapters['getAutoEvaluateFieldMappingConfig'] =
  () =>
    Promise.resolve({
      success: false,
      fieldMapping: {},
    });

const getDataReflowFieldMappingConfig: ObservationTraceAdapters['getDataReflowFieldMappingConfig'] =
  () =>
    Promise.resolve({
      success: false,
      fieldMapping: {},
      fieldMappings: [],
    });

const datasetSelect = (_props: {
  pageSize?: number;
  publishOption?: number;
  showSecurityLevel?: boolean;
  hiddenL4Dataset?: boolean;
  [key: string]: unknown;
}): React.ReactElement | null => null;

export const observationTraceAdapters: ObservationTraceAdapters = {
  getAutoEvaluateFieldMappingConfig,
  getDataReflowFieldMappingConfig,
  useFetchSpanDetail,
  DatasetSelect: datasetSelect,
  CozeloopTraceSelector,
  CozeloopTraceSelectorInitValues,
  TraceDetailPanel,
  TraceDetail,
};
