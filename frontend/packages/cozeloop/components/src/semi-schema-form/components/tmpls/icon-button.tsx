// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import {
  type FormContextType,
  type IconButtonProps,
  type RJSFSchema,
  type StrictRJSFSchema,
  TranslatableString,
} from '@rjsf/utils';
import {
  IconCozCopy,
  IconCozLongArrowUp,
  IconCozPlusCircle,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  IconButton,
  type IconButtonProps as SemiIconBaseButtonProps,
} from '@coze-arch/coze-design';

// The `type` and `color` for IconButtonProps collides with props of `ButtonProps` so omit it to avoid Typescript issue
export type BaseIconButtonProps<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
> = Omit<IconButtonProps<T, S, F>, 'type' | 'color'>;
type IconButtonComponent = <
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(
  props: BaseIconButtonProps<T, S, F>,
) => JSX.Element;

export default function BaseIconButton<
  T = unknown,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = object,
>(props: BaseIconButtonProps<T, S, F> & SemiIconBaseButtonProps) {
  const { iconType, icon, onClick, uiSchema, registry, style, ...otherProps } =
    props;

  return (
    <IconButton onClick={onClick} icon={icon} {...otherProps} size="small" />
  );
}

export const AddButton: IconButtonComponent = props => {
  const {
    registry: { translateString },
  } = props;
  return (
    <BaseIconButton
      title={translateString(TranslatableString.AddItemButton)}
      {...props}
      icon={<IconCozPlusCircle />}
    />
  );
};

export const CopyButton: IconButtonComponent = props => {
  const {
    registry: { translateString },
  } = props;
  return (
    <BaseIconButton
      title={translateString(TranslatableString.CopyButton)}
      {...props}
      icon={<IconCozCopy />}
    />
  );
};

export const MoveDownButton: IconButtonComponent = props => {
  const {
    registry: { translateString },
  } = props;
  return (
    <BaseIconButton
      title={translateString(TranslatableString.MoveDownButton)}
      {...props}
      icon={<IconCozLongArrowUp className="rotate-180" />}
    />
  );
};

export const MoveUpButton: IconButtonComponent = props => {
  const {
    registry: { translateString },
  } = props;
  return (
    <BaseIconButton
      title={translateString(TranslatableString.MoveUpButton)}
      {...props}
      icon={<IconCozLongArrowUp />}
    />
  );
};

export const RemoveButton: IconButtonComponent = props => {
  const {
    registry: { translateString },
  } = props;
  return (
    <BaseIconButton
      title={translateString(TranslatableString.RemoveButton)}
      {...props}
      type="danger"
      icon={<IconCozTrashCan />}
    />
  );
};
