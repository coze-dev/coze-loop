export const mockTraceDetailData = {
  msg: '',
  spans: [
    {
      status_code: 0,
      type: 'model',
      duration: '2729',
      input: `{
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "text": "思考1+1",
          "type": "text"
        }
      ]
    }
  ],
  "model": "gpt-4.1-mini-2025-04-14",
  "max_tokens": 1000,
  "extra_downstream_info": {
    "biz_uuid": "",
    "exercise_name": ""
  }
}`,
      span_name: 'model',
      started_at: '1755247286838',
      output:
        '{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"有点好奇你一直发“1111”是不是在回应我之前说的内容，表示认同呀，还是它在你们特定的交流里有独特的含义，可以跟我说说不？ "}}]}',
      parent_id: '0',
      span_id: '3044157d26f5265e',
      status: 'success',
      system_tags: {
        clip_fields: '[]',
        create_time: '1755247291583730',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
        tenant: 'fornax_saas',
      },
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
      custom_tags: {
        enterprise_id: '',
        input_tokens: '321',
        output_tokens: '42',
        prompt_key: 'playground-3199998098422608',
        stream: 'true',
        tenant: 'fornax_saas',
        tokens: '363',
        user_id: '3199998098422608',
      },
      logic_delete_date: '1755506491583',
      span_type: 'model',
    },
    {
      span_type: 'prompt_hub',
      started_at: '1755247286864',
      status_code: 0,
      type: 'unknwon',
      custom_tags: {
        user_id: '3199998098422608',
        enterprise_id: '',
        prompt_key: 'playground-3199998098422608',
        tenant: 'fornax_saas',
      },
      span_id: '71995eeafb662adf',
      duration: '0',
      span_name: 'PromptHub',
      status: 'success',
      system_tags: {
        clip_fields: '[]',
        create_time: '1755247288472630',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
        tenant: 'fornax_saas',
      },
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
      input: '{"prompt_key":"playground-3199998098422608","prompt_version":""}',
      logic_delete_date: '1755506488472',
      output:
        '{"workspace_id":7451587518450974760,"prompt_key":"playground-3199998098422608","version":"","prompt_template":{"template_type":"normal","messages":[{"role":"system","content":"1111"}]},"tool_call_config":{"tool_choice":"auto"},"llm_config":{"model_id":1737521813,"max_tokens":4096,"temperature":1}}',
      parent_id: '3044157d26f5265e',
    },
    {
      type: 'unknwon',
      custom_tags: {
        user_id: '3199998098422608',
        enterprise_id: '',
        prompt_key: 'playground-3199998098422608',
        tenant: 'fornax_saas',
      },
      duration: '2672',
      output:
        '{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"有点好奇你一直发“1111”是不是在回应我之前说的内容，表示认同呀，还是它在你们特定的交流里有独特的含义，可以跟我说说不？ "}}]}',
      parent_id: '3044157d26f5265e',
      status: 'success',
      system_tags: {
        create_time: '1755247291583694',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
        tenant: 'fornax_saas',
        clip_fields: '[]',
      },
      input:
        '{"prompt_variables":[],"messages":[{"role":"assistant","content":"11111"},{"role":"assistant","content":"不太明确你发的“11111”具体是什么意思呢。它可能是随意输入的一串数字，也可能代表着特定的含义，比如在某些交流场景中表示认同、知晓等。你可以给我解释一下，这样我就能更好地和你交流啦。 "},{"role":"assistant","content":"11111"},{"role":"assistant","content":"感觉你发这串数字好像有特别的想法，但我不太能猜透呢。你能不能多给我点提示，说说这“11111”背后的意思呀？ "},{"role":"assistant","content":"hello world"},{"role":"assistant","content":"“Hello, world”可是编程界的经典问候语呢，很多程序员在学习一门新编程语言时，第一个程序往往就是输出“Hello, world” 。它就像是开启编程世界大门的一把小钥匙。你是在学习编程，还是单纯分享这句经典话语呀？ "},{"role":"assistant","content":"1111"},{"role":"assistant","content":"在不同的语境里，“1111”有不同含义。在网络交流中，它常被用来简洁地表示“明白、同意、知晓”；在数字象征上，1通常代表着独立、开始、进取等含义。你发的“1111”是想表达哪种意思，还是它有特别的指代呢？ "},{"role":"user","content":"1111"}],"prompt_template":[{"role":"system","content":"1111"}]}',
      span_name: 'Sequence',
      span_type: 'sequence',
      status_code: 0,
      logic_delete_date: '1755506491583',
      span_id: '4f9641b3d7ac25c0',
      started_at: '1755247286873',
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
    },
    {
      logic_delete_date: '1755506488472',
      parent_id: '4f9641b3d7ac25c0',
      type: 'prompt',
      span_type: 'prompt',
      status_code: 0,
      system_tags: {
        clip_fields: '[]',
        create_time: '1755247288472691',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
        tenant: 'fornax_saas',
      },
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
      duration: '0',
      input:
        '{"templates":[{"role":"system","content":"1111"},{"role":"assistant","content":"11111"},{"role":"assistant","content":"不太明确你发的“11111”具体是什么意思呢。它可能是随意输入的一串数字，也可能代表着特定的含义，比如在某些交流场景中表示认同、知晓等。你可以给我解释一下，这样我就能更好地和你交流啦。 "},{"role":"assistant","content":"11111"},{"role":"assistant","content":"感觉你发这串数字好像有特别的想法，但我不太能猜透呢。你能不能多给我点提示，说说这“11111”背后的意思呀？ "},{"role":"assistant","content":"hello world"},{"role":"assistant","content":"“Hello, world”可是编程界的经典问候语呢，很多程序员在学习一门新编程语言时，第一个程序往往就是输出“Hello, world” 。它就像是开启编程世界大门的一把小钥匙。你是在学习编程，还是单纯分享这句经典话语呀？ "},{"role":"assistant","content":"1111"},{"role":"assistant","content":"在不同的语境里，“1111”有不同含义。在网络交流中，它常被用来简洁地表示“明白、同意、知晓”；在数字象征上，1通常代表着独立、开始、进取等含义。你发的“1111”是想表达哪种意思，还是它有特别的指代呢？ "},{"role":"user","content":"1111"}],"arguments":[]}',
      span_id: '69e356616e9cf386',
      span_name: 'PromptTemplate',
      custom_tags: {
        enterprise_id: '',
        prompt_key: 'playground-3199998098422608',
        tenant: 'fornax_saas',
        user_id: '3199998098422608',
      },
      output:
        '{"prompts":[{"role":"system","content":"1111"},{"role":"assistant","content":"11111"},{"role":"assistant","content":"不太明确你发的“11111”具体是什么意思呢。它可能是随意输入的一串数字，也可能代表着特定的含义，比如在某些交流场景中表示认同、知晓等。你可以给我解释一下，这样我就能更好地和你交流啦。 "},{"role":"assistant","content":"11111"},{"role":"assistant","content":"感觉你发这串数字好像有特别的想法，但我不太能猜透呢。你能不能多给我点提示，说说这“11111”背后的意思呀？ "},{"role":"assistant","content":"hello world"},{"role":"assistant","content":"“Hello, world”可是编程界的经典问候语呢，很多程序员在学习一门新编程语言时，第一个程序往往就是输出“Hello, world” 。它就像是开启编程世界大门的一把小钥匙。你是在学习编程，还是单纯分享这句经典话语呀？ "},{"role":"assistant","content":"1111"},{"role":"assistant","content":"在不同的语境里，“1111”有不同含义。在网络交流中，它常被用来简洁地表示“明白、同意、知晓”；在数字象征上，1通常代表着独立、开始、进取等含义。你发的“1111”是想表达哪种意思，还是它有特别的指代呢？ "},{"role":"user","content":"1111"}]}',
      started_at: '1755247286873',
      status: 'success',
    },
    {
      output:
        '{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"有点好奇你一直发“1111”是不是在回应我之前说的内容，表示认同呀，还是它在你们特定的交流里有独特的含义，可以跟我说说不？ "}}]}',
      type: 'model',
      custom_tags: {
        model_identification: '1737521813',
        model_provider: 'llm_gateway',
        output_tokens: '42',
        user_id: '3199998098422608',
        call_options: '{"temperature":1,"max_tokens":4096}',
        input_tokens: '321',
        latency_first_resp: '1413',
        model_id: '1737521813',
        tokens: '363',
        enterprise_id: '',
        model_name: '豆包·1.5·Pro·32k',
        stream: 'true',
        tenant: 'fornax_saas',
      },
      span_id: '53cc989d9b473cf7',
      span_type: 'model',
      status_code: 0,
      system_tags: {
        tenant: 'fornax_saas',
        clip_fields: '[]',
        create_time: '1755247291728194',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
      },
      duration: '2664',
      logic_delete_date: '1755506491728',
      started_at: '1755247286880',
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
      input:
        '{"messages":[{"role":"system","content":"1111"},{"role":"assistant","content":"11111"},{"role":"assistant","content":"不太明确你发的“11111”具体是什么意思呢。它可能是随意输入的一串数字，也可能代表着特定的含义，比如在某些交流场景中表示认同、知晓等。你可以给我解释一下，这样我就能更好地和你交流啦。 "},{"role":"assistant","content":"11111"},{"role":"assistant","content":"感觉你发这串数字好像有特别的想法，但我不太能猜透呢。你能不能多给我点提示，说说这“11111”背后的意思呀？ "},{"role":"assistant","content":"hello world"},{"role":"assistant","content":"“Hello, world”可是编程界的经典问候语呢，很多程序员在学习一门新编程语言时，第一个程序往往就是输出“Hello, world” 。它就像是开启编程世界大门的一把小钥匙。你是在学习编程，还是单纯分享这句经典话语呀？ "},{"role":"assistant","content":"1111"},{"role":"assistant","content":"在不同的语境里，“1111”有不同含义。在网络交流中，它常被用来简洁地表示“明白、同意、知晓”；在数字象征上，1通常代表着独立、开始、进取等含义。你发的“1111”是想表达哪种意思，还是它有特别的指代呢？ "},{"role":"user","content":"1111"}]}',
      parent_id: '4f9641b3d7ac25c0',
      span_name: '豆包·1.5·Pro·32k',
      status: 'success',
    },
  ],
  traces_advance_info: {
    tokens: {
      output: '0',
      input: '0',
    },
    trace_id: '78cec9bbb9edda421caedaaab65fb334',
  },
  code: 0,
};

export const mockTraceListData = {
  spans: [
    {
      logic_delete_date: '1755506491583',
      status: 'success',
      trace_id: '78cec9bbb9edda421caedaaab65fb334',
      parent_id: '0',
      span_id: '3044157d26f5265e',
      span_name: 'PromptExecutor',
      status_code: 0,
      system_tags: {
        tenant: 'fornax_saas',
        clip_fields: '[]',
        create_time: '1755247291583730',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
      },
      type: 'unknwon',
      duration: '2729',
      output:
        '{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"有点好奇你一直发“1111”是不是在回应我之前说的内容，表示认同呀，还是它在你们特定的交流里有独特的含义，可以跟我说说不？ "}}]}',
      custom_tags: {
        prompt_key: 'playground-3199998098422608',
        stream: 'true',
        tenant: 'fornax_saas',
        tokens: '363',
        user_id: '3199998098422608',
        enterprise_id: '',
        input_tokens: '321',
        output_tokens: '42',
      },
      input:
        '{"prompt_key":"playground-3199998098422608","prompt_version":"","prompt_variables":[],"messages":[{"role":"assistant","content":"11111"},{"role":"assistant","content":"不太明确你发的“11111”具体是什么意思呢。它可能是随意输入的一串数字，也可能代表着特定的含义，比如在某些交流场景中表示认同、知晓等。你可以给我解释一下，这样我就能更好地和你交流啦。 "},{"role":"assistant","content":"11111"},{"role":"assistant","content":"感觉你发这串数字好像有特别的想法，但我不太能猜透呢。你能不能多给我点提示，说说这“11111”背后的意思呀？ "},{"role":"assistant","content":"hello world"},{"role":"assistant","content":"“Hello, world”可是编程界的经典问候语呢，很多程序员在学习一门新编程语言时，第一个程序往往就是输出“Hello, world” 。它就像是开启编程世界大门的一把小钥匙。你是在学习编程，还是单纯分享这句经典话语呀？ "},{"role":"assistant","content":"1111"},{"role":"assistant","content":"在不同的语境里，“1111”有不同含义。在网络交流中，它常被用来简洁地表示“明白、同意、知晓”；在数字象征上，1通常代表着独立、开始、进取等含义。你发的“1111”是想表达哪种意思，还是它有特别的指代呢？ "},{"role":"user","content":"1111"}]}',
      span_type: 'prompt_executor',
      started_at: '1755247286838',
    },
    {
      span_id: '5993f10a2e84251c',
      type: 'unknwon',
      parent_id: '0',
      span_name: 'PromptExecutor',
      span_type: 'prompt_executor',
      status: 'success',
      status_code: 0,
      custom_tags: {
        prompt_key: 'playground-3199998098422608',
        stream: 'true',
        tenant: 'fornax_saas',
        tokens: '111',
        user_id: '3199998098422608',
        enterprise_id: '',
        input_tokens: '18',
        output_tokens: '93',
      },
      duration: '2657',
      input:
        '{"prompt_key":"playground-3199998098422608","prompt_version":"","prompt_variables":[],"messages":[{"role":"user","content":"11111"}]}',
      logic_delete_date: '1755491251307',
      output:
        '{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"Hello! It looks like you\'ve shared a string of 1s. Is there something specific you\'re thinking about or need help with? Feel free to let me know—I\'m here to assist!"}}]}',
      started_at: '1755232047815',
      system_tags: {
        tenant: 'fornax_saas',
        clip_fields: '[]',
        create_time: '1755232051307421',
        runtime:
          '{"language":"go","scene":"custom","loop_sdk_version":"v0.1.7"}',
      },
      trace_id: '001c96774d59c161628437b84d3ac1ef',
    },
  ],
  code: 0,
  msg: '',
  has_more: false,
  next_page_token:
    'eyJTdGFydFRpbWUiOjE3NTUyMzIwNDc4MTU5MjgsIlNwYW5JRCI6IjU5OTNmMTBhMmU4NDI1MWMifQ==',
};
