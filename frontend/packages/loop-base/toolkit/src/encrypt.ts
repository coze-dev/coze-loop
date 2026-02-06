// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
// URL安全的 Base64 编码/解码
const urlSafeBase64 = {
  encode: (str: string) => {
    // 将字符串转换为 UTF-8 编码
    const utf8Bytes = new TextEncoder().encode(str);
    // 将 UTF-8 字节转换为二进制字符串
    const binaryStr = String.fromCharCode(...utf8Bytes);
    // Base64 编码并替换特殊字符
    return btoa(binaryStr)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
  },
  decode: (str: string) => {
    str = str.replace(/-/g, '+').replace(/_/g, '/');
    while (str.length % 4) {
      str += '=';
    }
    // Base64 解码
    const binaryStr = atob(str);
    // 转换回 UTF-8 字节数组
    const bytes = new Uint8Array(binaryStr.length);
    for (let i = 0; i < binaryStr.length; i++) {
      bytes[i] = binaryStr.charCodeAt(i);
    }
    // 解码 UTF-8 字节数组为字符串
    return new TextDecoder().decode(bytes);
  },
};

// 简单的混淆密钥
const SALT = 'fornax';

export const encryptParams = (id?: string | number, name?: string) => {
  if (!id || !name) {
    return '';
  }

  // 将数据转换为对象并序列化
  const data = JSON.stringify({ i: id, n: name });

  // 简单混淆
  const mixed = data
    .split('')
    .map((char, index) => {
      const saltChar = SALT[index % SALT.length];
      return String.fromCharCode(char.charCodeAt(0) ^ saltChar.charCodeAt(0));
    })
    .join('');

  return urlSafeBase64.encode(mixed);
};

export const decryptParams = (str: string) => {
  try {
    // 解码
    const mixed = urlSafeBase64.decode(str);

    // 解混淆
    const data = mixed
      .split('')
      .map((char, index) => {
        const saltChar = SALT[index % SALT.length];
        return String.fromCharCode(char.charCodeAt(0) ^ saltChar.charCodeAt(0));
      })
      .join('');

    const { i: id, n: name } = JSON.parse(data);
    return { id, name };
  } catch (e) {
    console.error('解密参数失败:', e);
    return { id: '', name: '' };
  }
};
