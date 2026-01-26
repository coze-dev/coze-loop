namespace go stone.fornax.ml_flow.domain.trainingdatasetv2t

include "datasetv2.thrift"

struct TrainingDataset {
    1: optional datasetv2.Dataset dataset
    2: optional UsageScene usageScene
    3: optional list<DataFormat> dataFormats
}

typedef string UsageScene (ts.enum="true")
const UsageScene TextGenerationSft = "text_generation_sft"
const UsageScene TextGenerationDPOOrSimPO = "text_generation_dpo_or_simpo"
const UsageScene TextGenerationKTO = "text_generation_kto"
const UsageScene TextGenerationContinuePretrain = "text_generation_continue_pretrain"
const UsageScene TextGenerationRft = "text_generation_rft"

typedef string DataFormat (ts.enum="true")
const DataFormat WithoutToolCall = "without_tool_call"
const DataFormat WithToolCall = "with_tool_call"
const DataFormat ChosenAndRejected = "chosen_and_rejected"
const DataFormat ChosenOrRejected = "chosen_or_rejected"
const DataFormat OnlyText = "only_text"
const DataFormat RFTQueriesWithoutToolCall = "rft_queries_without_tool_call"
const DataFormat RFTQueriesWithToolCall = "rft_queries_with_tool_call"