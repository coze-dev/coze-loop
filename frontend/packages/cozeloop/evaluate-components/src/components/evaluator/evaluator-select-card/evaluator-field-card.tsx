// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { forwardRef, useImperativeHandle, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type Evaluator,
  type EvaluatorVersion,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozArrowRight,
  IconCozTrashCan,
  IconCozWarningCircleFillPalette,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Tooltip,
  useFieldState,
  useFormApi,
  withField,
} from '@coze-arch/coze-design';

import { EvaluatorVersionSelect } from '../../selectors/evaluator-version-select';
import { EvaluatorSelect } from '../../selectors/evaluator-select';
import { EvaluatorPreview } from '../../previews/evaluator-preview';

export interface EvaluatorFieldMappingValue {
  evaluator_id?: string;
  evaluator_version_id?: string;
}
interface FieldMappingCardProps {
  title?: React.ReactNode;
  content: React.ReactNode;
  hasError?: boolean;
  disableDelete?: boolean;
  open?: boolean;
  className?: string;
  onOpen?: (open: boolean) => void;
  onDelete?: () => void;
}

const FormEvaluatorSelect = withField(EvaluatorSelect);
const FormEvaluatorVersionSelect = withField(EvaluatorVersionSelect);

function FieldMappingCard({
  title,
  content,
  hasError,
  open = true,
  disableDelete,
  className,
  onOpen,
  onDelete,
}: FieldMappingCardProps) {
  return (
    <div
      className={classNames(
        'group border border-solid coz-stroke-primary rounded-[6px]',
        className,
      )}
    >
      <div
        className="h-11 px-4 flex flex-row items-center coz-bg-primary rounded-t-[6px] cursor-pointer"
        onClick={() => onOpen?.(!open)}
      >
        <div className="flex flex-row items-center flex-1 text-sm font-semibold coz-fg-plus">
          {title}
          <IconCozArrowRight
            className={classNames(
              'ml-1 h-4 w-4 coz-fg-primary transition-transform',
              open ? 'rotate-90' : '',
            )}
          />
          {hasError && !open ? (
            <IconCozWarningCircleFillPalette className="ml-1 w-4 h-4 coz-fg-hglt-red" />
          ) : null}
        </div>
        <div
          onClick={e => e.stopPropagation()}
          className="flex flex-row items-center gap-1 invisible group-hover:visible"
        >
          <Tooltip
            content={I18n.t('space_member_role_type_del_btn')}
            theme="dark"
          >
            <Button
              color="secondary"
              size="small"
              className="!h-6 user-select-none"
              icon={<IconCozTrashCan className="h-4 w-4" />}
              disabled={disableDelete}
              onClick={e => {
                e.stopPropagation();
                onDelete?.();
              }}
            />
          </Tooltip>
        </div>
      </div>
      <div className={open ? 'px-4' : 'hidden'}>{content}</div>
    </div>
  );
}

interface EvaluatorFieldCardProps {
  value?: EvaluatorFieldMappingValue;
  onChange?: (value: EvaluatorFieldMappingValue) => void;
  prefix?: string;
  content?:
    | React.ReactNode
    | ((
        evaluator: Evaluator | undefined,
        version: EvaluatorVersion | undefined,
      ) => React.ReactNode);
  initValues?: EvaluatorFieldMappingValue;
  defaultTitle?: React.ReactNode;
  hasError?: boolean;
  disabledVersionIds?: string[];
  disabled?: boolean;
  onDelete?: () => void;
  onEvaluatorVersionChange?: (version: EvaluatorVersion | undefined) => void;
}

export interface EvaluatorFieldCardRef {
  setOpen: (open: boolean) => void;
}

export const EvaluatorFieldCard = forwardRef<
  EvaluatorFieldCardRef,
  EvaluatorFieldCardProps
>(
  (
    {
      value,
      prefix,
      content,
      initValues,
      defaultTitle = I18n.t('evaluator'),
      hasError,
      disabledVersionIds,
      disabled,
      onDelete,
      onEvaluatorVersionChange,
    },
    ref,
  ) => {
    const { spaceID } = useSpace();
    const [open, setOpen] = useState(true);
    const [evaluator, setEvaluator] = useState<Evaluator | undefined>();
    const [evaluatorVersion, setEvaluatorVersion] = useState<
      EvaluatorVersion | undefined
    >();
    const formApi = useFormApi();
    const fieldState = useFieldState(`${prefix}`);

    // 初始化获取评估器和版本
    useRequest(async () => {
      const initVal = value ?? initValues;
      const evaluatorId = initVal?.evaluator_id;
      const versionId = initVal?.evaluator_version_id;
      if (!evaluatorId) {
        return;
      }
      const [evaluatorRes, versionRes] = await Promise.all([
        StoneEvaluationApi.BatchGetEvaluators({
          workspace_id: spaceID,
          evaluator_ids: [evaluatorId],
        })?.then(res => res?.evaluators?.[0]),
        versionId
          ? StoneEvaluationApi.ListEvaluatorVersions({
              workspace_id: spaceID,
              evaluator_id: evaluatorId,
              page_size: 200,
            })?.then(res =>
              res?.evaluator_versions?.find(e => e.id === versionId),
            )
          : Promise.resolve(undefined),
      ]);
      if (evaluatorRes) {
        setEvaluator(evaluatorRes);
      }
      if (versionRes) {
        setEvaluatorVersion(versionRes);
      }
    });

    useImperativeHandle(ref, () => ({
      setOpen: val => setOpen(val),
    }));

    // 用户行为变更导致版本的变化才用这个
    const handleVersionChange = (version: EvaluatorVersion | undefined) => {
      setEvaluatorVersion(version);
      onEvaluatorVersionChange?.(version);
    };

    const evaluatorContent = (
      <div>
        <div className="flex flex-row gap-5">
          <div className="flex-1 w-0">
            <FormEvaluatorSelect
              className="w-full"
              field={`${prefix}.evaluator_id`}
              fieldStyle={{ paddingBottom: 16 }}
              label={I18n.t('name')}
              placeholder={I18n.t('please_select_evaluator')}
              onChangeWithObject={false}
              disabled={disabled}
              rules={[
                { required: true, message: I18n.t('please_select_evaluator') },
              ]}
              onSelect={(_, option) => {
                setEvaluator(option);
                handleVersionChange(undefined);
                formApi?.setValue(`${prefix}.evaluator_version_id`, undefined);
              }}
            />
          </div>
          <div className="flex-1 w-0 flex flex-row">
            <div className="flex-1 relative">
              <FormEvaluatorVersionSelect
                className="w-full"
                field={`${prefix}.evaluator_version_id`}
                onChangeWithObject={false}
                variableRequired={true}
                label={I18n.t('version')}
                placeholder={I18n.t('please_select_a_version_number')}
                rules={[
                  {
                    required: true,
                    message: I18n.t('please_select_a_version_number'),
                  },
                ]}
                // 这个不能从value里取，响应式更新不及时
                evaluatorId={fieldState?.value?.evaluator_id}
                disabledVersionIds={disabledVersionIds}
                disabled={disabled}
                onSelect={(_, option) => {
                  handleVersionChange(option);
                }}
                onClear={() => {
                  handleVersionChange(undefined);
                }}
              />
            </div>
          </div>
        </div>
      </div>
    );

    const title =
      evaluator || evaluatorVersion ? (
        <EvaluatorPreview
          evaluator={{
            ...(evaluator || {}),
            current_version: evaluatorVersion,
          }}
        />
      ) : (
        defaultTitle
      );

    return (
      <FieldMappingCard
        title={title}
        open={open}
        hasError={hasError}
        onOpen={setOpen}
        disableDelete={disabled}
        onDelete={onDelete}
        content={
          <>
            {evaluatorContent}
            {typeof content === 'function'
              ? content(evaluator, evaluatorVersion)
              : content}
          </>
        }
      />
    );
  },
);
