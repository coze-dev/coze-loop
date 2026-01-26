// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { EvaluatorType } from '@cozeloop/api-schema/evaluation';
import { IconCozCode, IconCozAiFill } from '@coze-arch/coze-design/icons';

interface EvaluatorIconProps {
  evaluatorType?: EvaluatorType;
  iconSize?: number;
}

const EvaluatorIcon = (props: EvaluatorIconProps) => {
  const { evaluatorType = EvaluatorType.Prompt, iconSize = 14 } = props;

  const iconSizeStyle = useMemo(
    () => ({
      width: `${iconSize}px`,
      height: `${iconSize}px`,
      minWidth: `${iconSize}px`,
      minHeight: `${iconSize}px`,
    }),
    [iconSize],
  );

  const icon = useMemo(() => {
    if (evaluatorType === EvaluatorType.Code) {
      return (
        <IconCozCode style={iconSizeStyle} color="var(--coz-fg-secondary)" />
      );
    }
    if (evaluatorType === EvaluatorType.Prompt) {
      return (
        <IconCozAiFill style={iconSizeStyle} color="var(--coz-fg-secondary)" />
      );
    }
    return null;
  }, [evaluatorType, iconSizeStyle]);

  return icon;
};

export default EvaluatorIcon;
