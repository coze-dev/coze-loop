namespace go coze.loop.foundation.openapi

include "../../../base.thrift"

include "../extra.thrift"

struct UploadLoopFileRequest {
    1: required string              content_type (api.header="Content-Type") // file type
    2: required binary              body         (api.raw_body='')           // binary data

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct UploadLoopFileResponse {
    1: optional i32                 code
    2: optional string              msg
    3: optional FileData            data

    255: base.BaseResp BaseResp
}

struct FileData {
    1: optional i64 bytes (api.js_conv='true', go.tag='json:"bytes"')
    2: optional string file_name
}

service FoundationOpenAPIService {
    UploadLoopFileResponse UploadLoopFile(1: UploadLoopFileRequest req) (api.post='/v1/loop/files/upload') // for open api, etc sdk
}
