// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type LocalStorageKeys, cacheConfig } from './config';
interface CozeLoopStorageConfig {
  // 业务域，唯一标识
  field: string;
}

export class CozeLoopStorage {
  private field: string;
  private static userID: string;
  private static spaceID: string;

  static setUserID(id: string) {
    CozeLoopStorage.userID = id;
  }
  static setSpaceID(id: string) {
    CozeLoopStorage.spaceID = id;
  }
  constructor(config: CozeLoopStorageConfig) {
    this.field = config.field;
  }

  private makeKey(key: LocalStorageKeys) {
    let result = `[${this.field}]`;

    if (cacheConfig[key]?.bindAccount) {
      result += `:[${CozeLoopStorage.userID}]`;
    }
    if (cacheConfig[key]?.bindSpace) {
      result += `:[${CozeLoopStorage.spaceID}]`;
    }

    return `${result}:${key}`;
  }
  setItem(key: LocalStorageKeys, value: string) {
    localStorage.setItem(`${this.makeKey(key)}`, value);
  }

  getItem(key: LocalStorageKeys) {
    return localStorage.getItem(`${this.makeKey(key)}`);
  }

  removeItem(key: LocalStorageKeys) {
    localStorage.removeItem(`${this.makeKey(key)}`);
  }

  getKey(key: LocalStorageKeys) {
    return this.makeKey(key);
  }
}
