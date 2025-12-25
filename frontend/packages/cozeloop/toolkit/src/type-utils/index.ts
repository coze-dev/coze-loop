// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * 通用对象类型
 *
 * 避免业务直接使用 object、{}、Record<string, any> 还需要解决 lint 报错
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Obj = Record<string, any>;

/**
 * 获取类型 T 中所有可选属性的键
 *
 * @example
 * type A = GetOptionalKeys<{ a: string; b?: number }>;
 * //   ^? 'b'
 * type B = GetOptionalKeys<{ a: string | undefined; b: number; c?: string }>;
 * //   ^? 'a' | 'c'
 */
export type GetOptionalKeys<T> = {
  [K in keyof T]-?: T[K] extends infer P
    ? P extends undefined
      ? K
      : never
    : never;
}[keyof T];

/**
 * 检查类型 T 中是否所有属性都是可选的
 *
 * @example
 * type A = CheckIsAllOptional<{ a: string; b?: number }>;
 * //   ^? false
 * type B = CheckIsAllOptional<{ a: string | undefined; b?: string }>;
 * //   ^? true
 */
export type CheckIsAllOptional<T> =
  GetOptionalKeys<T> extends keyof T
    ? keyof T extends GetOptionalKeys<T>
      ? true
      : false
    : false;

/**
 * 展示对象完整类型
 *
 * @example
 * type Intersection = { a: string } & { b: number };
 * type Result = Expand<Intersection>;
 * // Result: { a: string; b: number }
 */
export type Expand<T extends Obj> = T extends infer U
  ? { [K in keyof U]: U[K] }
  : never;

/**
 * 只对特定字段做 required，常用于修正服务端类型声明错误
 *
 * @example
 * interface Agent {
 *  id?: string;
 *  name?: string;
 *  desc?: string
 * }
 * type Result = PartialRequired<Agent, 'id' | 'name'>;
 */
export type PartialRequired<T extends Obj, K extends keyof T> = Expand<
  {
    [P in K]-?: T[P];
  } & Pick<T, Exclude<keyof T, K>>
>;
