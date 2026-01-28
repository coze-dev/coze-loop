namespace go volcengine.agentkit.llm

include "volcengine.agentkit.llm.commercial.thrift"

service LLMCommercialService extends volcengine.agentkit.llm.commercial.LLMCommercialService{} (agw.js_conv = 'str', agw.cli_conv = 'str', api.tag = 'volc-agentkit-service')
