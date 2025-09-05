import { useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { DEFAULT_TEXT_STRING_SCHEMA } from '@cozeloop/evaluate-components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import { IconCozArrowRight } from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';

import { type EvaluatorPro } from '@/types/experiment/experiment-create';
import { ReadonlyMappingItem } from '@/components/mapping-item-field/readonly-mapping-item';

import { OpenDetailButton } from './open-detail-button';

// eslint-disable-next-line complexity
export function EvaluateItemRender({
  evaluatorPro,
}: {
  evaluatorPro: EvaluatorPro;
}) {
  const { baseURL } = useBaseURL();
  const [open, setOpen] = useState(true);
  const inputSchemas =
    evaluatorPro?.evaluatorVersionDetail?.evaluator_content?.input_schemas ??
    [];
  return (
    <div className="border border-solid coz-stroke-primary rounded-[6px]">
      <div
        className="h-11 px-4 flex flex-row items-center coz-bg-primary cursor-pointer"
        onClick={() => setOpen(pre => !pre)}
      >
        <div className="flex flex-row items-center flex-1 text-sm font-semibold coz-fg-plus gap-1">
          {evaluatorPro?.evaluator?.name}
          {evaluatorPro?.evaluatorVersion?.version ? (
            <Tag color="primary" className="!h-5 !px-2 !py-[2px] rounded-[3px]">
              {evaluatorPro.evaluatorVersion.version}
            </Tag>
          ) : null}

          <OpenDetailButton
            url={`${baseURL}/evaluation/evaluators/${
              evaluatorPro?.evaluator?.evaluator_id
            }?version=${evaluatorPro?.evaluatorVersion?.id}`}
          />

          <IconCozArrowRight
            className={classNames(
              'h-4 w-4 coz-fg-primary transition-transform',
              open ? 'rotate-90' : '',
            )}
          />
        </div>
      </div>

      <div className={open ? 'p-4' : 'hidden'}>
        <div className="text-sm font-medium coz-fg-primary mb-2">
          {I18n.t('field_mapping')}
        </div>
        <div className="flex flex-col gap-3">
          {inputSchemas.map(schema => (
            <ReadonlyMappingItem
              key={schema?.key}
              keyTitle={I18n.t('evaluator')}
              keySchema={{
                name: schema?.key,
                ...DEFAULT_TEXT_STRING_SCHEMA,
                content_type: schema.support_content_types?.[0],
                text_schema: schema.json_schema,
              }}
              optionSchema={evaluatorPro.evaluatorMapping?.[schema?.key ?? '']}
            />
          ))}
          {/* {Object.entries(evaluatorPro.evaluatorMapping || {}).map(([k, v]) => (
            <ReadonlyMappingItem
              key={k}
              keyTitle={I18n.t('evaluator')}
              keySchema={{
                name: k,
                ...DEFAULT_TEXT_STRING_SCHEMA,
              }}
              optionSchema={v}
            />
          ))} */}
        </div>
      </div>
    </div>
  );
}
