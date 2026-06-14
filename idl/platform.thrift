struct ListProviderReposReq {
    1: string provider (api.path="provider")
    2: optional i64 page (api.query="page")
    3: optional i64 per_page (api.query="per_page")
}

struct RepoItem {
    1: string id (api.body="id")
    2: string full_name (api.body="full_name")
    3: string name (api.body="name")
    4: string owner (api.body="owner")
    5: optional string description (api.body="description")
    6: optional string default_branch (api.body="default_branch")
    7: bool private (api.body="private")
}

struct ListProviderReposResp {
    1: list<RepoItem> items (api.body="items")
}

struct ListProviderBranchesReq {
    1: string provider (api.path="provider")
    2: string owner (api.path="owner")
    3: string repo (api.path="repo")
}

struct BranchItem {
    1: string name (api.body="name")
}

struct ListProviderBranchesResp {
    1: list<BranchItem> items (api.body="items")
}

struct GetBranchSHAReq {
    1: string provider (api.path="provider")
    2: string owner (api.path="owner")
    3: string repo (api.path="repo")
    4: string branch (api.query="branch")
}

struct GetBranchSHAResp {
    1: string sha (api.body="sha")
}

struct ListProviderPullsReq {
    1: string provider (api.path="provider")
    2: string owner (api.path="owner")
    3: string repo (api.path="repo")
    4: optional string state (api.query="state")
    5: optional i64 page (api.query="page")
    6: optional i64 per_page (api.query="per_page")
}

struct PRAuthor {
    1: string username (api.body="username")
    2: optional string name (api.body="name")
    3: optional string avatar_url (api.body="avatar_url")
}

struct PullItem {
    1: i64 number (api.body="number")
    2: string title (api.body="title")
    3: string state (api.body="state")
    4: string source_branch (api.body="source_branch")
    5: string target_branch (api.body="target_branch")
    6: optional string head_sha (api.body="head_sha")
    7: optional string base_sha (api.body="base_sha")
    8: optional string web_url (api.body="web_url")
    9: optional string created_at (api.body="created_at")
    10: optional string updated_at (api.body="updated_at")
    11: optional PRAuthor author (api.body="author")
}

struct ListProviderPullsResp {
    1: list<PullItem> items (api.body="items")
}
