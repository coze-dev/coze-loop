// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';

import IconButtonContainer from '../id-render/icon-button-container';

export default function JumpIconButton(
  props: {
    className?: string;
    style?: React.CSSProperties;
  } & React.DOMAttributes<HTMLDivElement>,
) {
  return <IconButtonContainer {...props} icon={<IconCozLongArrowTopRight />} />;
}
