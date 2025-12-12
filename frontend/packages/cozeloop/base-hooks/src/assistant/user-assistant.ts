// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
const OPEN_ASSISTANT_EVENT = 'ai_assistant_open';

const CLOSE_ASSISTANT_EVENT = 'ai_assistant_close';

const SEND_MESSAGE_EVENT = 'ai_assistant_send_message';

const emitter = new EventTarget();

function emit(eventName: string, detail?: unknown) {
  emitter.dispatchEvent(new CustomEvent(eventName, { detail }));
}

function on(eventName: string, callback: (e: Event) => void) {
  emitter.addEventListener(eventName, callback);
}
function off(eventName: string, callback: (e: Event) => void) {
  emitter.removeEventListener(eventName, callback);
}

class Assistant {
  /** 是否启用助手 */
  isEnable = false;
  /** 启用助手 */
  enableAssistant = () => {
    if (this.isEnable) {
      return;
    }
    this.isEnable = true;
  };
  /** 打开助手 */
  open = (params?: { from: string }) => {
    emit(OPEN_ASSISTANT_EVENT, params);
  };
  /** 监听助手打开事件, 返回取消监听函数 */
  onOpen = (callback: (params?: { from: string }) => void) => {
    const cb = (e?: Event) => {
      callback((e as CustomEvent).detail);
    };
    on(OPEN_ASSISTANT_EVENT, cb);
    return () => {
      off(OPEN_ASSISTANT_EVENT, cb);
    };
  };
  /** 关闭助手 */
  close = () => {
    emit(CLOSE_ASSISTANT_EVENT);
  };
  /** 监听助手关闭事件，返回取消监听函数 */
  onClose = (callback: () => void) => {
    on(CLOSE_ASSISTANT_EVENT, callback);
    return () => {
      off(CLOSE_ASSISTANT_EVENT, callback);
    };
  };
  sendMessage = (params: { query: string }) => {
    emit(SEND_MESSAGE_EVENT, params);
  };
  /** 监听助手发送消息事件， 返回取消监听函数 */
  onSendMessage = (callback: (params: { query: string }) => void) => {
    const cb = (e?: Event) => {
      callback((e as CustomEvent).detail);
    };
    on(SEND_MESSAGE_EVENT, cb);
    return () => {
      off(SEND_MESSAGE_EVENT, cb);
    };
  };
}

/** 用户助手 */
export const userAssistant = new Assistant();
