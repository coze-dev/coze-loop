/* eslint-disable security/detect-object-injection */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
import { useCallback, useRef, useState } from 'react';

import { cloneDeep } from 'lodash-es';
import { type ParsedEvent } from 'eventsource-parser';
import { I18n } from '@cozeloop/i18n-adapter';
import { type ContentPartLoop } from '@cozeloop/components';
import { ContentType, type DebugToolCall } from '@cozeloop/api-schema/prompt';
import { promptDebug } from '@cozeloop/api-schema';
import { fetchStream } from '@coze-arch/fetch-stream';
import { Toast } from '@coze-arch/coze-design';
import { useSmoothText } from '@bytedance/calypso';

import { messageId } from '@/utils/prompt';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';

export enum RespondingStatus {
  /** 响应中 */
  Responding = 'Responding',
  /** 点击按钮后等待响应 */
  Starting = 'Starting',
  /** 未生成 */
  Inactive = 'Inactive',
  /** 失败 */
  Error = 'Error',
}

export interface CostInfoProps {
  characters?: number;
  outpotTokens?: string;
  duration?: string;
  inputTokens?: string;
  recordId?: string;
}

export interface LLMStreamResponse {
  message: string;
  tools?: Array<DebugToolCall>;
  debugTrace?: string;
  costInfo?: CostInfoProps;
  debugId?: Int64;
  reasoningContent?: string;
  parts?: Array<ContentPartLoop>;
}

const RISE_ERR_CODE = 601505011;
const JINJA_ERR_CODES = [600501004, 600501005];

export const useLLMStreamRun = (uid?: number, abortTeaSend?: () => void) => {
  const {
    mockTools: draftMockTools,
    compareConfig,
    userDebugConfig,
  } = usePromptMockDataStore.getState();
  const mockTools = uid
    ? compareConfig?.groups?.[uid]?.debug_core?.mock_tools
    : draftMockTools;

  const singleStepDebug = compareConfig?.groups?.length
    ? false
    : userDebugConfig?.single_step_debug;

  const respondingStatusRef = useRef<RespondingStatus>(
    RespondingStatus.Inactive,
  );
  const [respondingStatus, setRespondingStatus] = useState<RespondingStatus>(
    RespondingStatus.Inactive,
  );

  const streamTools = useRef<Array<DebugToolCall>>([]);

  const [autoExecuteResult, setAutoExecuteResult] = useState('');
  const autoExecuteResultRef = useRef('');

  const [reasoningContent, setReasoningContent] = useState('');
  const reasoningContentRef = useRef('');

  const stepDebuggingTraceRef = useRef<string>();
  const stepDebuggingContentRef = useRef<string>();
  const debugIdRef = useRef<Int64>();
  const { text: smoothExecuteResult } = useSmoothText(
    autoExecuteResult,
    respondingStatusRef.current === RespondingStatus.Responding,
  );

  const { text: reasoningContentResult } = useSmoothText(
    reasoningContent,
    respondingStatusRef.current === RespondingStatus.Responding,
  );

  const costInfoRef = useRef<CostInfoProps>({
    characters: 0,
    outpotTokens: '0',
    duration: '0',
    inputTokens: '0',
    recordId: '',
  });

  const abortControllerRef = useRef<AbortController>();

  const currentTextPartRef = useRef<ContentPartLoop | null>(null);
  const fileParts = useRef<Array<ContentPartLoop>>([]);

  const mergeTextContent = useCallback((content: string) => {
    if (!content) {
      return;
    }

    if (!currentTextPartRef.current) {
      const newTextPart: ContentPartLoop = {
        type: ContentType.Text,
        text: content,
        uid: messageId(),
      };
      currentTextPartRef.current = newTextPart;
      fileParts.current = [...fileParts.current, newTextPart];
    } else {
      currentTextPartRef.current.text =
        (currentTextPartRef.current.text || '') + content;
      fileParts.current = fileParts.current.map(part => {
        if (
          part.uid === currentTextPartRef.current?.uid &&
          currentTextPartRef.current
        ) {
          return currentTextPartRef.current;
        }
        return part;
      });
    }
  }, []);

  const resetInfo = () => {
    autoExecuteResultRef.current = '';
    setAutoExecuteResult('');
    costInfoRef.current = {
      characters: 0,
      outpotTokens: '0',
      duration: '0',
      inputTokens: '0',
    };
    streamTools.current = [];
    stepDebuggingTraceRef.current = '';
    stepDebuggingContentRef.current = '';
    debugIdRef.current = undefined;
    reasoningContentRef.current = '';
    setReasoningContent('');
    setRespondingStatus(RespondingStatus.Inactive);
  };

  const abort = () => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = undefined;
    stepDebuggingTraceRef.current = undefined;
    respondingStatusRef.current = RespondingStatus.Inactive;
    setRespondingStatus(RespondingStatus.Inactive);
    abortTeaSend?.();
  };

  const startStream = useCallback(
    (
      params: promptDebug.DebugStreamingRequest,
      stepDebug?: boolean,
      onStartSuccess?: () => void,
    ): Promise<LLMStreamResponse> => {
      if (
        respondingStatusRef.current !== RespondingStatus.Inactive &&
        respondingStatusRef.current !== RespondingStatus.Error
      ) {
        return Promise.resolve({ message: '' });
      }
      if (!stepDebug) {
        streamTools.current = [];
      }

      autoExecuteResultRef.current = '';
      reasoningContentRef.current = '';
      setAutoExecuteResult('');
      setReasoningContent('');
      respondingStatusRef.current = RespondingStatus.Starting;
      setRespondingStatus(RespondingStatus.Starting);
      if (!params?.debug_trace_key) {
        costInfoRef.current = {
          characters: 0,
          outpotTokens: '0',
          duration: '0',
          inputTokens: '0',
        };
      }
      stepDebuggingTraceRef.current = '';
      stepDebuggingContentRef.current = '';
      abortControllerRef.current = new AbortController();
      const startTime = new Date().getTime();
      debugIdRef.current = undefined;

      return new Promise<LLMStreamResponse>((resolve, reject) => {
        fetchStream<ParsedEvent>(
          promptDebug.DebugStreaming.meta.url.replace(
            ':prompt_id',
            params.prompt?.id || 'playground',
          ),
          {
            method: 'POST',
            headers: {
              'content-type': 'application/json',
              'Agw-Js-Conv': 'str',
            },
            body: JSON.stringify(params),
            signal: abortControllerRef.current?.signal,
            onStart() {
              fileParts.current = [];
              currentTextPartRef.current = null;
              respondingStatusRef.current = RespondingStatus.Responding;
              setRespondingStatus(RespondingStatus.Responding);
              onStartSuccess?.();
              return Promise.resolve();
            },
            onMessage({ message }) {
              try {
                const messageChunk = JSON.parse(
                  message.data,
                ) as promptDebug.DebugStreamingResponse & {
                  msg?: string;
                  message?: string;
                  biz_extra?: {
                    biz_err_custom_extra?: string;
                  };
                  biz_status?: number;
                };

                if (!messageChunk?.delta) {
                  const bizExtra =
                    messageChunk?.biz_extra?.biz_err_custom_extra;
                  const extra = JSON.parse(bizExtra || '{}');
                  debugIdRef.current = extra?.debug_id;

                  const riskContent = JSON.parse(extra?.risk_content || '{}');

                  const onlyBackUp = (
                    riskContent?.recommend_operations || []
                  ).every(item => item === 'backup_response');

                  if (
                    RISE_ERR_CODE === messageChunk?.biz_status &&
                    onlyBackUp
                  ) {
                    autoExecuteResultRef.current = I18n.t(
                      'prompt_user_or_model_contains_risky_content',
                    );
                    setAutoExecuteResult(autoExecuteResultRef.current);
                    resolve({
                      message: autoExecuteResultRef.current,
                      tools: [],
                      debugTrace: undefined,
                      costInfo: {
                        characters: 0,
                        outpotTokens: '0',
                        duration: '0',
                        inputTokens: '0',
                        recordId: '',
                      },
                      debugId: debugIdRef.current,
                      reasoningContent: undefined,
                    });
                    return;
                  }

                  throw new Error(
                    messageChunk?.msg ||
                      messageChunk?.message ||
                      I18n.t('model_run_error'),
                    {
                      cause: messageChunk?.biz_status || '',
                    },
                  );
                }
                const {
                  tool_calls,
                  content = '',
                  reasoning_content = '',
                  parts = [],
                } = messageChunk.delta;

                autoExecuteResultRef.current =
                  autoExecuteResultRef.current + content;
                setAutoExecuteResult(autoExecuteResultRef.current);
                reasoningContentRef.current =
                  reasoningContentRef.current + reasoning_content;
                setReasoningContent(reasoningContentRef.current);
                stepDebuggingContentRef.current =
                  stepDebuggingContentRef.current + content;
                if (
                  stepDebuggingTraceRef.current &&
                  stepDebuggingTraceRef.current !== messageChunk.debug_trace_key
                ) {
                  stepDebuggingContentRef.current = '';
                  stepDebuggingTraceRef.current = '';
                }

                if (tool_calls?.length && singleStepDebug) {
                  stepDebuggingTraceRef.current = messageChunk.debug_trace_key;
                }

                tool_calls?.forEach(toolCall => {
                  const functionName = toolCall.function_call?.name;
                  const mockResp = mockTools?.find(
                    mt => mt.name === functionName,
                  )?.mock_response;

                  const toolIdx = streamTools.current.findIndex(
                    it =>
                      it.tool_call?.index === toolCall?.index &&
                      it.debug_trace_key === messageChunk.debug_trace_key,
                  );
                  if (toolIdx < 0) {
                    streamTools.current.push({
                      tool_call: toolCall,
                      mock_response: mockResp,
                      debug_trace_key: messageChunk.debug_trace_key,
                    });
                  } else {
                    const oldToolList = streamTools.current.slice();
                    const oldTool = cloneDeep(oldToolList[toolIdx]);
                    if (oldTool?.tool_call?.function_call) {
                      const { arguments: oldArguments } =
                        oldTool.tool_call.function_call;
                      const newArguments =
                        oldArguments +
                        (toolCall?.function_call?.arguments || '');
                      oldTool.tool_call.function_call.arguments = newArguments;
                      oldToolList[toolIdx] = {
                        ...oldTool,
                      };
                      streamTools.current = [...oldToolList];
                    }
                  }
                });

                if (content) {
                  mergeTextContent(content);
                }

                if (parts) {
                  currentTextPartRef.current = null;
                  fileParts.current = [...fileParts.current, ...parts];
                }

                debugIdRef.current = messageChunk?.debug_id;
                costInfoRef.current = {
                  ...costInfoRef.current,
                  characters: autoExecuteResultRef.current.length,
                  outpotTokens: `${
                    Number(costInfoRef.current.outpotTokens || 0) +
                    Number(messageChunk.usage?.output_tokens || 0)
                  }`,

                  inputTokens: `${
                    Number(costInfoRef.current.inputTokens || 0) +
                    Number(messageChunk.usage?.input_tokens || 0)
                  }`,
                };
              } catch (error) {
                respondingStatusRef.current = RespondingStatus.Error;
                setRespondingStatus(RespondingStatus.Error);

                console.error(error);

                if ((error as Error)?.cause === RISE_ERR_CODE) {
                  Toast.error(
                    I18n.t('prompt_user_or_model_contains_risky_content'),
                  );
                  reject(error);
                } else if (
                  JINJA_ERR_CODES.includes(Number((error as Error)?.cause))
                ) {
                  Toast.error((error as Error)?.message);
                  resolve({
                    debugId: debugIdRef.current,
                    message: autoExecuteResultRef.current,
                    tools: streamTools.current,
                  });
                } else {
                  Toast.error(I18n.t('model_runtime_error'));
                  resolve({
                    debugId: debugIdRef.current,
                    message: autoExecuteResultRef.current,
                    tools: streamTools.current,
                  });
                }
              }
            },
            onAllSuccess: () => {
              const endTime = new Date().getTime();

              costInfoRef.current = {
                ...costInfoRef.current,
                duration: `${(
                  Number(costInfoRef.current.duration || 0) +
                  Number(endTime - startTime)
                ).toFixed(0)}`,
              };

              respondingStatusRef.current = RespondingStatus.Inactive;
              setRespondingStatus(RespondingStatus.Inactive);
              resolve({
                message: autoExecuteResultRef.current,
                tools: streamTools.current,
                debugTrace: stepDebuggingTraceRef.current,
                costInfo: costInfoRef.current,
                debugId: debugIdRef.current,
                reasoningContent: reasoningContentRef.current,
                parts: fileParts.current.some(
                  it => it.type !== ContentType.Text,
                )
                  ? fileParts.current
                  : [],
              });
            },
            onError: e => {
              Toast.error(
                e?.fetchStreamError?.msg || I18n.t('model_runtime_error'),
              );
              console.error(e?.fetchStreamError?.msg);
            },
          },
        );
      });
    },
    [mockTools, singleStepDebug],
  );

  const streamRefTools = streamTools.current;
  const stepDebuggingTrace = stepDebuggingTraceRef.current;
  const costInfo = costInfoRef.current;
  const stepDebuggingContent = stepDebuggingContentRef.current;
  const debugId = debugIdRef.current;

  return {
    respondingStatus,
    autoExecuteResult,
    smoothExecuteResult,
    streamRefTools,
    costInfo,
    startStream,
    abort,
    stepDebuggingTrace,
    stepDebuggingContent,
    debugId,
    reasoningContentResult,
    resetInfo,
  };
};

export function isResponding(status?: RespondingStatus): boolean {
  switch (status) {
    case RespondingStatus.Responding:
    case RespondingStatus.Starting:
      return true;
    case RespondingStatus.Error:
    case RespondingStatus.Inactive:
    default:
      return false;
  }
}
