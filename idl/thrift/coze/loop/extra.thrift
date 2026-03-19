namespace go coze.loop.extra

struct Extra {
    1: optional string src (api.header="src")
    2: optional string user_agent (api.header="user-agent")
}
