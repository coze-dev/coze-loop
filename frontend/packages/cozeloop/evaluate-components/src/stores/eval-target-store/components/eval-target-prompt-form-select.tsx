import {
  type CommonFieldProps,
  type SelectProps,
  withField,
} from '@coze-arch/coze-design';

import { PromptEvalTargetSelect } from '../../../components/selectors/evaluate-target';

const FormSelectInner = withField(PromptEvalTargetSelect);

const PromptEvalTargetFormSelect: React.FC<
  SelectProps & CommonFieldProps
> = props => (
  <FormSelectInner
    remote
    onChangeWithObject
    label="Prompt key"
    rules={[{ required: true, message: '请选择Prompt key' }]}
    placeholder="请选择Prompt key"
    showCreateBtn={true}
    {...props}
  />
);

export default PromptEvalTargetFormSelect;
