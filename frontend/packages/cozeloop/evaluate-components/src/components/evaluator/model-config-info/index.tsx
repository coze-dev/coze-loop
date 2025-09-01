import { type ModelConfig } from '@cozeloop/api-schema/evaluation';

import { EvaluateModelConfigEditor } from '@/components/evaluate-model-config-editor';

export function ModelConfigInfo({ data }: { data?: ModelConfig }) {
  return (
    <>
      <div className="text-sm font-medium coz-fg-primary mb-2">{'模型'}</div>
      {data ? (
        <EvaluateModelConfigEditor
          value={data}
          disabled={true}
          popoverProps={{ position: 'bottomRight' }}
        />
      ) : (
        '-'
      )}
    </>
  );
}
