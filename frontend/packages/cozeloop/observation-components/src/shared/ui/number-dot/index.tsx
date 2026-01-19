// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
interface NumberDotProps {
  count: number;
  color?: 'brand' | 'error';
}

export const NumberDot = ({ count, color = 'brand' }: NumberDotProps) => (
  <div
    className={`flex items-center text-[13px] font-medium justify-center w-[20px] h-[20px] rounded-[50%] ${color === 'brand' ? 'bg-brand-4 text-brand-9' : 'bg-[#FFEBE9] text-[#D0292F]'} `}
  >
    {count}
  </div>
);
