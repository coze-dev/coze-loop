// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useMemo, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluatorSelect,
  EvaluatorVersionSelect,
  getEvaluatorJumpUrl,
} from '@cozeloop/evaluate-components';
import { useOpenWindow, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import {
  IconCozArrowRight,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  type RuleItem,
  Tag,
  Tooltip,
  useFieldApi,
  useFieldState,
  withField,
} from '@coze-arch/coze-design';

import { type EvaluatorPro } from '@/types/experiment/experiment-create';
import { getEvaluatorVersion } from '@/request/evaluator';
import { ReactComponent as ErrorIcon } from '@/assets/icon-alert.svg';

import { OpenDetailText } from './open-detail-text';
import { EvaluatorFieldItemSynthe } from './evaluator-field-item-synthe';

const FormEvaluatorSelect = withField(EvaluatorSelect);
const FormEvaluatorVersionSelect = withField(EvaluatorVersionSelect);

interface EvaluatorFieldItemProps {
  arrayField: {
    key: string;
    field: string;
    remove: () => void;
  };
  index: number;
  evaluationSetSchemas?: FieldSchema[];
  evaluateTargetSchemas?: FieldSchema[];
  selectedVersionIds?: string[];
  getEvaluatorMappingFieldRules?: (k: FieldSchema) => RuleItem[];
}

export function EvaluatorFieldItem(props: EvaluatorFieldItemProps) {
  const {
    arrayField,
    index,
    evaluationSetSchemas,
    evaluateTargetSchemas,
    selectedVersionIds,
    getEvaluatorMappingFieldRules,
  } = props;

  const { spaceID } = useSpace();
  const [open, setOpen] = useState(true);
  const evaluatorProFieldState = useFieldState(arrayField.field);
  const evaluatorPro = evaluatorProFieldState.value as EvaluatorPro;
  const evaluatorProApi = useFieldApi(arrayField.field);

  const { openBlank } = useOpenWindow();

  const { evaluator } = evaluatorPro;

  const versionId = evaluatorPro?.evaluatorVersion?.id;
  const versionDetailService = useRequest(
    async () => {
      if (
        !versionId ||
        evaluatorPro?.evaluatorVersionDetail?.id === versionId
      ) {
        return evaluatorPro?.evaluatorVersionDetail;
      }

      const res = await getEvaluatorVersion({
        workspace_id: spaceID,
        evaluator_version_id: versionId,
      });
      const resVersion = res.evaluator?.current_version;
      const currentVersionID = (evaluatorProApi.getValue() as EvaluatorPro)
        .evaluatorVersion?.id;
      if (currentVersionID && currentVersionID === resVersion?.id) {
        evaluatorProApi.setValue({
          ...evaluatorProApi.getValue(),
          evaluatorVersionDetail: resVersion,
        });
      }
    },
    {
      ready: Boolean(versionId),
      refreshDeps: [versionId],
    },
  );

  const jumpUrl = useMemo(
    () =>
      getEvaluatorJumpUrl({
        evaluatorType: evaluator?.evaluator_type,
        evaluatorId: evaluator?.evaluator_id,
        evaluatorVersionId: evaluatorPro?.evaluatorVersion?.id,
      }),
    [
      evaluator?.evaluator_id,
      evaluator?.evaluator_type,
      evaluatorPro?.evaluatorVersion?.id,
    ],
  );

  useEffect(() => {
    if (evaluatorProFieldState.error) {
      setOpen(true);
    }
  }, [evaluatorProFieldState.error]);

  return (
    <>
      <div className="group border border-solid coz-stroke-primary rounded-[6px]">
        <div
          className="h-11 px-4 flex flex-row items-center coz-bg-primary rounded-t-[6px] cursor-pointer"
          onClick={() => setOpen(pre => !pre)}
        >
          <div className="flex flex-row items-center flex-1 text-sm font-semibold coz-fg-plus">
            <span className="truncate max-w-[698px]">
              {evaluatorPro?.evaluator?.name ||
                `${I18n.t('evaluator_placeholder1', { placeholder1: index + 1 })}`}
            </span>
            {evaluatorPro?.evaluatorVersion?.version ? (
              <Tag
                color="primary"
                className="!h-5 !px-2 !py-[2px] rounded-[3px] ml-1"
              >
                {evaluatorPro.evaluatorVersion.version}
              </Tag>
            ) : null}

            <IconCozArrowRight
              className={classNames(
                'ml-1 h-4 w-4 coz-fg-primary transition-transform',
                open ? 'rotate-90' : '',
              )}
            />

            {evaluatorProFieldState.error && !open ? (
              <ErrorIcon className="ml-1 w-4 h-4 coz-fg-hglt-red" />
            ) : null}
          </div>
          <div className="flex flex-row items-center gap-1 invisible group-hover:visible">
            <Tooltip content={I18n.t('delete')} theme="dark">
              <Button
                color="secondary"
                size="small"
                className="!h-6"
                icon={<IconCozTrashCan className="h-4 w-4" />}
                onClick={e => {
                  e.stopPropagation();
                  arrayField.remove();
                }}
              />
            </Tooltip>
          </div>
        </div>
        <div className={open ? 'px-4' : 'hidden'}>
          <div className="flex flex-row gap-5">
            <div className="flex-1 w-0">
              <FormEvaluatorSelect
                className="w-full"
                field={`${arrayField.field}.evaluator`}
                fieldStyle={{ paddingBottom: 16 }}
                label={I18n.t('name')}
                placeholder={I18n.t('please_select', { field: '' })}
                onChangeWithObject
                rules={[
                  {
                    required: true,
                    message: I18n.t('please_select', { field: '' }),
                  },
                ]}
                onChange={v => {
                  evaluatorProApi.setValue({
                    evaluator: v,
                    evaluatorVersion: undefined,
                  });
                }}
              />
            </div>
            <div className="flex-1 w-0 flex flex-row">
              <div className="flex-1 relative">
                <FormEvaluatorVersionSelect
                  className="w-full"
                  field={`${arrayField.field}.evaluatorVersion`}
                  onChangeWithObject
                  variableRequired={true}
                  label={{
                    text: I18n.t('version'),
                    className: 'justify-between pr-0',
                    extra: (
                      <>
                        {versionId ? (
                          <OpenDetailText
                            className="absolute right-0 top-2.5"
                            url={jumpUrl}
                            customOpen={() => openBlank(jumpUrl)}
                          />
                        ) : null}
                      </>
                    ),
                  }}
                  placeholder={I18n.t('please_select', { field: '' })}
                  rules={[
                    {
                      required: true,
                      message: I18n.t('please_select', { field: '' }),
                    },
                  ]}
                  evaluatorId={evaluatorPro?.evaluator?.evaluator_id}
                  disabledVersionIds={selectedVersionIds}
                />
              </div>
            </div>
          </div>

          <EvaluatorFieldItemSynthe
            arrayField={arrayField}
            evaluatorType={evaluator?.evaluator_type}
            loading={versionDetailService.loading}
            versionDetail={evaluatorPro?.evaluatorVersionDetail}
            evaluationSetSchemas={evaluationSetSchemas}
            evaluateTargetSchemas={evaluateTargetSchemas}
            getEvaluatorMappingFieldRules={getEvaluatorMappingFieldRules}
          />
        </div>
      </div>
    </>
  );
}
