struct ListContributorsReq {
    1: optional string provider (api.query="provider")
    2: optional string repo (api.query="repo")
    3: optional string from (api.query="from")
    4: optional string to (api.query="to")
    5: optional string sort (api.query="sort")
    6: optional string order (api.query="order")
    7: optional i64 page (api.query="page")
    8: optional i64 page_size (api.query="page_size")
}

struct ContributorAuthor {
    1: string username (api.body="username")
    2: optional string name (api.body="name")
    3: optional string avatar_url (api.body="avatar_url")
}

struct ContributorStats {
    1: i64 total_reviews (api.body="total_reviews")
    2: i64 total_findings (api.body="total_findings")
    3: double avg_score (api.body="avg_score")
    4: optional string last_review_at (api.body="last_review_at")
}

struct ContributorTrendPoint {
    1: string date (api.body="date")
    2: i64 reviews (api.body="reviews")
    3: i64 findings (api.body="findings")
}

struct ContributorItem {
    1: ContributorAuthor author (api.body="author")
    2: ContributorStats stats (api.body="stats")
    3: optional list<ContributorTrendPoint> trend (api.body="trend")
}

struct ListContributorsResp {
    1: list<ContributorItem> items (api.body="items")
    2: i64 total (api.body="total")
    3: i64 page (api.body="page")
    4: i64 page_size (api.body="page_size")
    5: i64 total_pages (api.body="total_pages")
}

struct GetContributorReq {
    1: string author (api.path="author")
    2: optional string provider (api.query="provider")
    3: optional string repo (api.query="repo")
    4: optional string from (api.query="from")
    5: optional string to (api.query="to")
}

struct ContributorDetail {
    1: ContributorAuthor author (api.body="author")
    2: ContributorStats stats (api.body="stats")
    3: optional list<ContributorTrendPoint> trend (api.body="trend")
}

struct GetContributorResp {
    1: ContributorDetail detail (api.body="detail")
}

struct GetContributorTrendReq {
    1: string author (api.path="author")
    2: optional string provider (api.query="provider")
    3: optional string repo (api.query="repo")
    4: optional string from (api.query="from")
    5: optional string to (api.query="to")
}

struct GetContributorTrendResp {
    1: list<ContributorTrendPoint> trend (api.body="trend")
}

struct CategoryStat {
    1: string category (api.body="category")
    2: i64 count (api.body="count")
}

struct GetContributorCategoriesReq {
    1: string author (api.path="author")
    2: optional string provider (api.query="provider")
    3: optional string repo (api.query="repo")
    4: optional string from (api.query="from")
    5: optional string to (api.query="to")
}

struct GetContributorCategoriesResp {
    1: list<CategoryStat> categories (api.body="categories")
}

struct ListProjectsReq {
    1: optional string provider (api.query="provider")
    2: optional string repo (api.query="repo")
    3: optional string from (api.query="from")
    4: optional string to (api.query="to")
    5: optional string sort (api.query="sort")
    6: optional string order (api.query="order")
    7: optional i64 page (api.query="page")
    8: optional i64 page_size (api.query="page_size")
}

struct ProjectSummary {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: i64 review_count (api.body="review_count")
    4: double avg_score (api.body="avg_score")
    5: double pass_rate (api.body="pass_rate")
    6: i64 total_findings (api.body="total_findings")
    7: i64 critical_count (api.body="critical_count")
    8: i64 major_count (api.body="major_count")
    9: i64 minor_count (api.body="minor_count")
    10: i64 info_count (api.body="info_count")
    11: string last_review_at (api.body="last_review_at")
}

struct ProjectDetail {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: i64 review_count (api.body="review_count")
    4: double avg_score (api.body="avg_score")
    5: double pass_rate (api.body="pass_rate")
    6: i64 total_findings (api.body="total_findings")
    7: i64 critical_count (api.body="critical_count")
    8: i64 major_count (api.body="major_count")
    9: i64 minor_count (api.body="minor_count")
    10: i64 info_count (api.body="info_count")
    11: string last_review_at (api.body="last_review_at")
    12: double median_score (api.body="median_score")
    13: i64 max_score (api.body="max_score")
    14: i64 min_score (api.body="min_score")
    15: map<string, i64> grade_distribution (api.body="grade_distribution")
    16: double avg_findings (api.body="avg_findings")
    17: double avg_changed_lines (api.body="avg_changed_lines")
}

struct ProjectTrendPoint {
    1: string period (api.body="period")
    2: double avg_score (api.body="avg_score")
    3: i64 review_count (api.body="review_count")
    4: i64 total_findings (api.body="total_findings")
}

struct FileRiskStat {
    1: string path (api.body="path")
    2: i64 count (api.body="count")
    3: i64 critical_count (api.body="critical_count")
    4: i64 major_count (api.body="major_count")
}

struct ListProjectsResp {
    1: list<ProjectSummary> projects (api.body="projects")
    2: i64 total (api.body="total")
    3: i64 page (api.body="page")
    4: i64 page_size (api.body="page_size")
}

struct GetProjectReq {
    1: string provider (api.query="provider")
    2: string repo (api.query="repo")
    3: optional string from (api.query="from")
    4: optional string to (api.query="to")
}

struct GetProjectResp {
    1: ProjectDetail detail (api.body="detail")
}

struct GetProjectTrendResp {
    1: list<ProjectTrendPoint> trend (api.body="trend")
}

struct GetProjectCategoriesResp {
    1: list<CategoryStat> categories (api.body="categories")
}

struct GetProjectFilesResp {
    1: list<FileRiskStat> files (api.body="files")
}

struct GetProjectContributorsReq {
    1: string provider (api.query="provider")
    2: string repo (api.query="repo")
    3: optional string from (api.query="from")
    4: optional string to (api.query="to")
    5: optional i64 limit (api.query="limit")
}

struct ContributorSummary {
    1: string author (api.body="author")
    2: string author_name (api.body="author_name")
    3: string author_avatar (api.body="author_avatar")
    4: i64 review_count (api.body="review_count")
    5: double avg_score (api.body="avg_score")
    6: double pass_rate (api.body="pass_rate")
    7: i64 total_findings (api.body="total_findings")
    8: i64 critical_count (api.body="critical_count")
    9: i64 major_count (api.body="major_count")
    10: i64 minor_count (api.body="minor_count")
    11: i64 info_count (api.body="info_count")
    12: string last_review_at (api.body="last_review_at")
}

struct GetProjectContributorsResp {
    1: list<ContributorSummary> contributors (api.body="contributors")
}

service statsService {
    ListContributorsResp ListContributors(1: ListContributorsReq req) (api.get="/api/v1/stats/contributors")
    GetContributorResp GetContributor(1: GetContributorReq req) (api.get="/api/v1/stats/contributors/:author")
    GetContributorTrendResp GetContributorTrend(1: GetContributorTrendReq req) (api.get="/api/v1/stats/contributors/:author/trend")
    GetContributorCategoriesResp GetContributorCategories(1: GetContributorCategoriesReq req) (api.get="/api/v1/stats/contributors/:author/categories")
    ListProjectsResp ListProjects(1: ListProjectsReq req) (api.get="/api/v1/stats/projects")
    GetProjectResp GetProject(1: GetProjectReq req) (api.get="/api/v1/stats/projects/detail")
    GetProjectTrendResp GetProjectTrend(1: GetProjectReq req) (api.get="/api/v1/stats/projects/trend")
    GetProjectCategoriesResp GetProjectCategories(1: GetProjectReq req) (api.get="/api/v1/stats/projects/categories")
    GetProjectFilesResp GetProjectFiles(1: GetProjectReq req) (api.get="/api/v1/stats/projects/files")
    GetProjectContributorsResp GetProjectContributors(1: GetProjectContributorsReq req) (api.get="/api/v1/stats/projects/contributors")
}
