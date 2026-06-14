struct ProjectAgentItem {
    1: i64 agent_id (api.body="agent_id")
    2: bool enabled (api.body="enabled")
    3: i64 priority (api.body="priority")
}

struct ProjectAgentConfig {
    1: i64 agent_id (api.body="agent_id")
    2: string name (api.body="name")
    3: string display_name (api.body="display_name")
    4: string description (api.body="description")
    5: string background (api.body="background")
    6: bool enabled (api.body="enabled")
    7: i64 priority (api.body="priority")
    8: bool project_enabled (api.body="project_enabled")
    9: i64 project_priority (api.body="project_priority")
}

struct GetProjectAgentsReq {
    1: string provider (api.query="provider")
    2: string repo (api.query="repo")
}

struct GetProjectAgentsResp {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: list<ProjectAgentConfig> agents (api.body="agents")
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
    3: list<ProjectAgentItem> agents (api.body="agents")
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
