// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type BannerProps } from '@coze-arch/coze-design';

export enum BenefitBannerScene {
  Global = 'global',
  PromptDetail = 'prompt-detail',
  EvaluatorDebug = 'evaluator-debug',
}

interface Props {
  closable?: boolean;
  className?: string;
  scene?: BenefitBannerScene;
}

// 跳转至火山引擎-费用中心的账号总览页面
export function BenefitBanner({
  closable,
  className,
  scene = BenefitBannerScene.Global,
}: Props) {
  return <></>;
}

export function BenefitBaseBanner({
  description,
  className,
  ...rest
}: BannerProps) {
  return <></>;
}
