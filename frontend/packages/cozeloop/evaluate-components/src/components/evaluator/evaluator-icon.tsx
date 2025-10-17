/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
