import { withField } from '@coze-arch/coze-design';

import { type BaseSelectProps } from './types';
import BaseSearchSelect from './base-search-select';

const BaseSearchFormSelect: React.FC<BaseSelectProps> = withField(
  (props: BaseSelectProps) => <BaseSearchSelect {...props} />,
);

export default BaseSearchFormSelect;
