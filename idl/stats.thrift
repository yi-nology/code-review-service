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
