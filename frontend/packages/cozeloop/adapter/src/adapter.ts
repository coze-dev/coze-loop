import type { Adapters } from './interfaces';

// 使用泛型类重构adapters
const adapterMap = new Map();

/**
 * 注册adapter
 * @param scope adapter 作用域
 * @param name adapter 名称
 * @param adapter adapter实例
 */
export const register = <S extends keyof Adapters, T extends keyof Adapters[S]>(
  scope: S,
  name: T,
  adapter: Adapters[S][T],
) => {
  const adapterName = `${scope}.${name as string}`;
  if (adapterMap.has(adapterName)) {
    throw new Error(`adapter ${adapterName} already registered`);
  }
  adapterMap.set(adapterName, adapter);
};

/**
 * 以 scope 维度批量注册 adapter
 * @param scope adapter作用域
 * @param adapters adapter实例
 */
export const registerScope = <S extends keyof Adapters>(
  scope: S,
  adapters: Adapters[S],
) => {
  Object.keys(adapters).forEach(name => {
    register(scope, name as keyof Adapters[S], adapters[name]);
  });
};

/**
 * 获取adapter，优先推荐使用 useAdapter
 * @param scope adapter作用域
 * @param name adapter名称
 * @returns adapter实例
 */
export const get = <S extends keyof Adapters, T extends keyof Adapters[S]>(
  scope: S,
  name: T,
) => {
  const adapterName = `${scope}.${name as string}`;
  return adapterMap.get(adapterName) as Adapters[S][T] | undefined;
};
