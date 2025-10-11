/**
 * 获取coze 相关地址信息
 * @returns
 */
export function useCozeLocation() {
  const cozeOrigin = window.location.origin.replace('loop.', '');

  return {
    origin: cozeOrigin,
  };
}
