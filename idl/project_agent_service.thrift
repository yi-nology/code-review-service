include "project_agent.thrift"

service projectAgentService {
    project_agent.GetProjectAgentsResp GetProjectAgents(1: project_agent.GetProjectAgentsReq req) (api.get="/api/v1/settings/project-agents")
    project_agent.UpdateProjectAgentResp UpdateProjectAgent(1: project_agent.UpdateProjectAgentReq req) (api.put="/api/v1/settings/project-agents")
    project_agent.BatchUpdateProjectAgentsResp BatchUpdateProjectAgents(1: project_agent.BatchUpdateProjectAgentsReq req) (api.post="/api/v1/settings/project-agents/batch")
    project_agent.DeleteProjectAgentResp DeleteProjectAgent(1: project_agent.DeleteProjectAgentReq req) (api.delete="/api/v1/settings/project-agents")
}
