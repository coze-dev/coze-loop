// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/// <reference types="@rsbuild/core/types" />
/// <reference types='@coze-arch/bot-typings' />

declare module '*.svg' {
  export const ReactComponent: React.FunctionComponent<
    React.SVGProps<SVGSVGElement>
  >;

  /**
   * The default export type depends on the svgDefaultExport config,
   * it can be a string or a ReactComponent
   * */
  const content: any;
  export default content;
}
declare type Int64 = string | number;

declare type RouteParams = Partial<
  Record<'appID' | 'graphID' | 'repoID', string>
>;

/// <reference types="@rsbuild/core/types" />
