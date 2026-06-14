struct GetProjectAgentsReq {
    1: string provider (api.query="provider")
    2: string repo (api.query="repo")
}

struct GetProjectAgentsResp {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: string agents (api.body="agents")
}

struct UpdateProjectAgentReq {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: i64 agent_id (api.body="agent_id")
    4: bool enabled (api.body="enabled")
    5: i64 priority (api.body="priority")
}

struct UpdateProjectAgentResp {
    1: string message (api.body="message")
}

struct BatchUpdateProjectAgentsReq {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: string agents (api.body="agents")
}

struct BatchUpdateProjectAgentsResp {
    1: string message (api.body="message")
    2: i64 count (api.body="count")
}

struct DeleteProjectAgentReq {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: i64 agent_id (api.body="agent_id")
}

struct DeleteProjectAgentResp {
    1: string message (api.body="message")
}
