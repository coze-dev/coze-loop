// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable max-lines-per-function */
/* eslint-disable security/detect-object-injection */
/* eslint-disable @coze-arch/max-line-per-function -- 监控逻辑较多，函数较长，后续可拆分 */
/* eslint-disable @typescript-eslint/no-explicit-any -- 表单组件需要处理任意类型的表单数据与第三方库接口 */
import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  useEffect,
} from 'react';
import type { Ref } from 'react';

import { nanoid } from 'nanoid';
import { debounce } from 'lodash-es';
import dayjs from 'dayjs';
import { useDebounceFn } from 'ahooks';
import { type BaseFormProps, Form, type FormApi } from '@coze-arch/coze-design';

import { useReportEvent } from '../provider';
import { EventNames } from './enum';

export interface SentinelFormProps<T extends Record<string, any>>
  extends BaseFormProps<any> {
  /**
   * 表单ID,用于快速定位该表单的用户，命名规范：[业务模块]-[业务名称]-[表单名称]
   * 示例：数据引擎-AIDP-表单创建
   */
  formID: string;
  // 用 T 强化对外体验，内部传递给实际 Form 时再做浅断言
  onSubmit?: (values: T, e?: React.FormEvent<HTMLFormElement>) => void;
  onValueChange?: (values: T, changed: Partial<T>) => void;
}
// 为外部可拿到的 ref 形态定义类型
export type SentinelFormRef<T extends Record<string, any>> = Form<T> & {
  submitLog?: (isInterfaceError?: boolean, error?: any) => Promise<void>;
};

export type SentinelFormApi<T extends Record<string, any>> = FormApi<T> & {
  submitLog?: (isInterfaceError?: boolean, error?: any) => Promise<void>;
};

const SentinelFormInner = <T extends Record<string, any>>(
  {
    formID,
    children,
    onValueChange,
    onSubmit,
    getFormApi,
    ...props
  }: SentinelFormProps<T>,
  ref: Ref<SentinelFormRef<T>>,
) => {
  const sendEvent = useReportEvent();
  const sessionID = useMemo(() => nanoid(), []);
  const formRef = useRef<any>(null);
  const instanceRef = useRef<any>(null);
  const startTimeRef = useRef(0);
  const interfaceErrorCount = useRef(0);
  const validateErrorCount = useRef(0);
  const firstValidateSuccess = useRef(true);
  const noOperation = useRef(true);
  const noSubmit = useRef(true);
  const submitSuccess = useRef(false);

  const validateErrorLog = debounce((error: any) => {
    firstValidateSuccess.current = false;
    validateErrorCount.current = validateErrorCount.current + 1;

    const validateErrorTime = Date.now();
    const errorKeys = Object.keys(error) || [];
    sendEvent(EventNames.LOOP_FORM_FIELD_VALIDATE_ERROR, {
      form_id: formID,
      session_id: sessionID,
      form_item_error_count: errorKeys.length,
      form_item_error_detail: JSON.stringify(error || {}),
      time: validateErrorTime,
      time_str: dayjs(validateErrorTime).format('YYYY-MM-DD HH:mm:ss'),
    });
  }, 500);

  // 提交日志：仅用于接口错误统计与成功上报，不处理校验错误
  const submitLog = useCallback(
    async (isInterfaceError?: boolean, error?: any) => {
      noSubmit.current = false;
      if (isInterfaceError) {
        firstValidateSuccess.current = false;
        interfaceErrorCount.current = interfaceErrorCount.current + 1;
        sendEvent?.(EventNames.LOOP_FORM_SUBMIT_INTERFACE_ERROR, {
          form_id: formID,
          session_id: sessionID,
          form_interface_error_detail: JSON.stringify(error || {}),
          time: Date.now(),
          time_str: dayjs(Date.now()).format('YYYY-MM-DD HH:mm:ss'),
        });
        return;
      }

      const values = await formRef?.current?.formApi?.getValues();
      submitSuccess.current = true;
      const formItemShow = Object.keys(values);
      const formItemFilled = formItemShow.filter(key => values[key]);
      const successTime = Date.now();
      sendEvent?.(EventNames.LOOP_FORM_SUBMIT_SUCCESS, {
        form_id: formID,
        session_id: sessionID,
        // 完成耗时
        form_complete_consume_time: Date.now() - startTimeRef.current,
        // 接口报错次数，需要计算
        form_interface_error: interfaceErrorCount.current,
        // 一次点击成功次数 pv
        form_once_success_submit:
          interfaceErrorCount.current === 0 && firstValidateSuccess.current,
        // 表单填写项
        form_item_filled: JSON.stringify(formItemFilled),
        // 表单展现项
        form_item_show: JSON.stringify(formItemShow),
        // 表单项使用率
        form_item_used_percent: formItemFilled.length / formItemShow.length,
        form_values: JSON.stringify(values),
        time: successTime,
        time_str: dayjs(successTime).format('YYYY-MM-DD HH:mm:ss'),
      });
    },
    [formID, sessionID],
  );

  // 劫持 validate，确保任何方式获取到的 formApi 都会命中被包装的 validate
  const hijackValidate = useCallback(
    (instance: any) => {
      if (!instance) {
        return instance;
      }
      const FLAG = '__validateHijacked__';
      if (instance[FLAG]) {
        return instance;
      }
      const originalValidate =
        typeof instance.validate === 'function'
          ? instance.validate.bind(instance)
          : null;
      if (originalValidate) {
        instance.validate = (...args: any[]) => {
          try {
            const maybePromise = originalValidate(...args);
            return Promise.resolve(maybePromise).catch(err => {
              // 校验失败：直接上报校验错误，不触发 submitLog
              validateErrorLog(err);
              throw err;
            });
          } catch (err) {
            validateErrorLog(err);
            throw err;
          }
        };
        instance[FLAG] = true;
      }
      if (!instance?.submitLog) {
        instance.submitLog = submitLog;
      }
      return instance;
    },
    [submitLog],
  );

  const operationTimelineRef = useRef({});
  // 关闭标签页/浏览器上报所需：保证拿到最新的 formID/sessionID 与避免重复上报
  const formIdRef = useRef<string>(formID);
  const sessionIdRef = useRef<string>(sessionID);
  const flushedRef = useRef(false);

  useEffect(() => {
    formIdRef.current = formID;
  }, [formID]);

  useEffect(() => {
    sessionIdRef.current = sessionID;
  }, [sessionID]);

  // 空闲状态相关
  const [isIdle, setIsIdle] = useState(false);
  const [idleStartTime, setIdleStartTime] = useState<number | null>(null);
  const IDLE_THRESHOLD = 5000; // 空闲阈值，单位毫秒（5秒）
  const IDLE_CHECK_INTERVAL = 1000; // 空闲检查间隔，单位毫秒（1秒）
  // 记录最后一次用户活跃时间
  const [lastActiveTime, setLastActiveTime] = useState(Date.now());

  const handleValueChange = changedValues => {
    noOperation.current = false;
    for (const key in changedValues) {
      if (Object.prototype.hasOwnProperty.call(changedValues, key)) {
        const currentTime = Date.now();
        setLastActiveTime(currentTime); // 更新最后活跃时间

        if (!operationTimelineRef.current[key]) {
          // 首次操作该字段，初始化操作记录
          operationTimelineRef.current = {
            ...operationTimelineRef.current,
            [key]: [
              {
                startTime: currentTime,
                startTimeStr: dayjs(currentTime).format('YYYY-MM-DD HH:mm:ss'),
                endTime: currentTime,
                endTimeStr: dayjs(currentTime).format('YYYY-MM-DD HH:mm:ss'),
                duration: 0,
                idleTime: 0, // 初始无效空闲时间为0
              },
            ],
          };
        } else {
          const lastOperation =
            operationTimelineRef.current[key][
              operationTimelineRef.current[key].length - 1
            ];
          const totalDurationSinceLast = currentTime - lastOperation.startTime; // 本次操作从上次开始到现在的总时长
          let idleTimeDeducted = 0;

          if (isIdle && idleStartTime !== null) {
            // 如果之前处于空闲状态，扣除这段空闲时间
            idleTimeDeducted = currentTime - idleStartTime;
            setIsIdle(false); // 重置空闲状态
          }

          const effectiveDuration = totalDurationSinceLast - idleTimeDeducted;
          const newOperation = {
            startTime: currentTime,
            startTimeStr: dayjs(currentTime).format('YYYY-MM-DD HH:mm:ss'),
            endTime: currentTime,
            endTimeStr: dayjs(currentTime).format('YYYY-MM-DD HH:mm:ss'),
            duration: effectiveDuration,
            idleTime: idleTimeDeducted,
          };

          // 更新操作时间轴，添加新的操作记录
          operationTimelineRef.current = {
            ...operationTimelineRef.current,
            [key]: [...operationTimelineRef.current[key], newOperation],
          };
        }
      }
    }
  };

  const onFormClose = () => {
    const closeTime = Date.now();
    sendEvent?.(EventNames.LOOP_FORM_CLOSE, {
      form_id: formIdRef.current,
      session_id: sessionIdRef.current,
      // 无表单操作 pv
      form_no_operation: noOperation.current,
      // 有表单操作，无表单提交操作次数 pv
      form_operation_no_submit: !noOperation.current && noSubmit.current,
      // 表单是否提交成功
      form_operation_submit_success: submitSuccess.current,
      // 接口报错次数​, 需要计算
      form_request_error_times: interfaceErrorCount.current,
      // 表单项报错次数
      form_item_error_times: validateErrorCount.current,
      time: closeTime,
      time_str: dayjs(closeTime).format('YYYY-MM-DD HH:mm:ss'),
    });
  };
  const onFormCloseDobounce = useDebounceFn(onFormClose, { wait: 200 });

  const onValuesChange = (values: any, changedValues: any) => {
    handleValueChange(changedValues);
    onValueChange?.(values as T, changedValues as Partial<T>);
  };

  useEffect(() => {
    if (sessionID && formID) {
      startTimeRef.current = Date.now();
    }
  }, [formID, sessionID]);

  // 定时检测用户是否进入空闲状态
  useEffect(() => {
    const idleChecker = setInterval(() => {
      if (!isIdle && Date.now() - lastActiveTime > IDLE_THRESHOLD) {
        setIsIdle(true);
        setIdleStartTime(Date.now());
      }
    }, IDLE_CHECK_INTERVAL); // 每秒检查一次

    return () => clearInterval(idleChecker);
  }, [isIdle, lastActiveTime]);

  // 初始化: 进入表单时间
  useEffect(() => {
    if (sessionID && formID) {
      const initTime = Date.now();
      sendEvent?.(EventNames.INIT_LOOP_FORM, {
        form_id: formID,
        session_id: sessionID,
        time: initTime,
        time_str: dayjs(initTime).format('YYYY-MM-DD HH:mm:ss'),
      });
    }

    return () => {
      if (formID && sessionID && !flushedRef.current) {
        flushedRef.current = true;
        sendEvent?.(EventNames.LOOP_FORM_FIELD_CHANGE_TIMELINE, {
          form_id: formID,
          session_id: sessionID,
          time_line: JSON.stringify(operationTimelineRef.current),
        });
        onFormCloseDobounce.run();
      }
    };
  }, [formID, sessionID]);

  // 关注页面关闭（tab 关闭/浏览器退出/导航离开等），不处理页面隐藏
  useEffect(() => {
    const flushOnClose = () => {
      if (flushedRef.current) {
        return;
      }
      const fid = formIdRef.current;
      const sid = sessionIdRef.current;
      if (!fid || !sid) {
        return;
      }
      flushedRef.current = true;

      sendEvent?.(EventNames.LOOP_FORM_FIELD_CHANGE_TIMELINE, {
        form_id: fid,
        session_id: sid,
        time_line: JSON.stringify(operationTimelineRef.current),
      });

      onFormCloseDobounce.run();
    };

    // 仅监听会导致页面真正卸载的事件
    window.addEventListener('pagehide', flushOnClose);
    window.addEventListener('beforeunload', flushOnClose);

    return () => {
      window.removeEventListener('pagehide', flushOnClose);
      window.removeEventListener('beforeunload', flushOnClose);
    };
  }, []);

  useImperativeHandle(ref, () => {
    const newFormApi: any = hijackValidate(formRef?.current?.formApi);
    const result: SentinelFormRef<T> = {
      ...(formRef?.current || {}),
      formApi: newFormApi,
      submitLog,
    };
    return result;
  });

  return (
    <Form
      ref={formRef}
      onValueChange={onValuesChange}
      getFormApi={instance => {
        const hijacked = hijackValidate(instance);
        instanceRef.current = hijacked;
        getFormApi?.(hijacked);
      }}
      onSubmit={values => {
        onSubmit?.(values as T);
      }}
      onSubmitFail={validateErrorLog}
      {...(props as BaseFormProps<any>)}
    >
      {children}
    </Form>
  );
};

export const SentinelForm = forwardRef(SentinelFormInner) as <
  T extends Record<string, any>,
>(
  props: SentinelFormProps<T> & { ref?: Ref<SentinelFormRef<T>> },
) => JSX.Element;
