import { I18n } from '@cozeloop/i18n-adapter';
import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import {
  type CommonFieldProps,
  type SelectProps,
  withField,
} from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '../../../types/evaluate-target';
import { PromptEvalTargetVersionSelect } from '../../../components/selectors/evaluate-target';
import { OpenDetailText } from '../../../components/common';

const FormSelectInner = withField(PromptEvalTargetVersionSelect);

interface EvalTargetVersionProps {
  promptId: string;
  sourceTargetVersion: string;
  onChange: (key: keyof CreateExperimentValues, value: unknown) => void;
}

const PromptEvalTargetVersionFormSelect: React.FC<
  SelectProps & CommonFieldProps & EvalTargetVersionProps
> = props => {
  const { promptId, sourceTargetVersion } = props;
  const { getPromptDetailURL } = useResourcePageJump();

  return (
    <FormSelectInner
      remote
      onChangeWithObject
      rules={[{ required: true, message: I18n.t('select_version') }]}
      label={{
        text: I18n.t('version'),
        className: 'justify-between pr-0',
        extra: (
          <>
            {promptId && sourceTargetVersion ? (
              <OpenDetailText
                url={getPromptDetailURL(promptId, sourceTargetVersion)}
              />
            ) : null}
          </>
        ),
      }}
      {...props}
    />
  );
};

export default PromptEvalTargetVersionFormSelect;
