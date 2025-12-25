// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable max-params */
import { useRef } from 'react';

import { isInteger, isUndefined } from 'lodash-es';
import classNames from 'classnames';
import {
  CozInputNumber,
  Slider,
  type SliderProps,
} from '@coze-arch/coze-design';

import styles from './index.module.less';

interface InputSliderProps {
  value?: number;
  onChange?: (v: number) => void;
  max?: number;
  min?: number;
  step?: number;
  disabled?: boolean;
  decimalPlaces?: number;
  marks?: SliderProps['marks'];
  className?: string;
}

export const formateDecimalPlacesString = (
  value: string | number,
  prevValue?: number,
  decimalPlaces?: number,
) => {
  if (isUndefined(decimalPlaces)) {
    return value.toString();
  }
  const numberValue = Number(value);
  const stringValue = value.toString();
  if (Number.isNaN(numberValue)) {
    return `${value}`;
  }
  if (decimalPlaces === 0 && !isInteger(Number(value)) && prevValue) {
    return `${prevValue}`;
  }
  const decimalPointIndex = stringValue.indexOf('.');

  if (decimalPointIndex < 0) {
    return stringValue;
  }
  const formattedValue = stringValue.substring(
    0,
    decimalPointIndex + 1 + decimalPlaces!,
  );

  if (formattedValue.endsWith('.') && decimalPlaces === 0) {
    return formattedValue.substring(0, formattedValue.length - 1);
  }
  return formattedValue;
};

function getDecimalPlaces(num) {
  if (!num) {
    return 0;
  }
  // 将数字转换为字符串
  const numStr = num.toString();
  // 检查是否包含小数点
  if (numStr.includes('.')) {
    // 使用正则表达式匹配小数点后面的数字
    const decimalPart = numStr.split('.')[1];
    return decimalPart.length; // 返回小数位数
  }
  return 0; // 如果没有小数点，返回0
}

const formateDecimalPlacesNumber = (
  value: number,
  prevValue?: number,
  decimalPlaces?: number,
  isPasted?: boolean,
) => {
  if (isUndefined(decimalPlaces)) {
    if (isPasted) {
      return value;
    }
    const currentDecimalPlaces = getDecimalPlaces(value);
    const prevDecimalPlaces = getDecimalPlaces(prevValue);
    if (
      prevDecimalPlaces === 2 &&
      currentDecimalPlaces > 2 &&
      !isUndefined(prevValue) &&
      Math.abs(prevValue! - value).toFixed(2) === '0.01'
    ) {
      const pow2 = Math.pow(10, 2);
      return Math.round(value * pow2) / pow2;
    }

    return value;
  }
  if (decimalPlaces === 0 && !isInteger(value) && prevValue) {
    return prevValue;
  }

  const pow = Math.pow(10, decimalPlaces!);
  return Math.round(value * pow) / pow;
};

export const InputSlider: React.FC<InputSliderProps> = ({
  value,
  onChange,
  max = 1,
  min = 0,
  step = 1,
  disabled,
  decimalPlaces,
  marks,
  className,
}) => {
  const isPasted = useRef(false);
  const onNumberChange = (numberValue: number) => {
    const formattedValue = formateDecimalPlacesNumber(
      numberValue,
      value,
      decimalPlaces,
      isPasted.current,
    );
    isPasted.current = false;
    onChange?.(formattedValue);
  };

  return (
    <div className={classNames(styles['input-slider'], className)}>
      <Slider
        key={`${min}-${max}`}
        disabled={disabled}
        value={value}
        max={max}
        min={min}
        step={step}
        marks={marks}
        onChange={v => {
          if (typeof v === 'number') {
            onChange?.(v);
          }
        }}
      />
      <CozInputNumber
        className={styles['input-number']}
        value={value}
        disabled={disabled}
        formatter={inputValue =>
          formateDecimalPlacesString(inputValue, value, decimalPlaces)
        }
        onNumberChange={onNumberChange}
        max={max}
        min={min}
        step={step}
        onPaste={() => (isPasted.current = true)}
      />
    </div>
  );
};
