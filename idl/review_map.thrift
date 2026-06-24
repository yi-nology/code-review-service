struct ReviewMapSummary {
    1: string id (api.body="id")
    2: string review_id (api.body="review_id")
    3: string session_id (api.body="session_id")
    4: string title (api.body="title")
    5: string description (api.body="description")
    6: string summary (api.body="summary")
    7: i64 file_count (api.body="file_count")
    8: i64 section_count (api.body="section_count")
    9: string created_at (api.body="created_at")
}

struct ReviewMapDetail {
    1: string id (api.body="id")
    2: string review_id (api.body="review_id")
    3: string session_id (api.body="session_id")
    4: string title (api.body="title")
    5: string description (api.body="description")
    6: string sections (api.body="sections")
    7: string dependencies (api.body="dependencies")
    8: string summary (api.body="summary")
    9: i64 file_count (api.body="file_count")
    10: i64 section_count (api.body="section_count")
    11: string mermaid (api.body="mermaid")
    12: string created_at (api.body="created_at")
}

struct GenerateMapReq {
    1: string id (api.path="id")
    2: string session_id (api.body="session_id")
    3: string diff (api.body="diff")
    4: optional string review_id (api.body="review_id")
}

struct GenerateMapResp {
    1: string map_data (api.body="map_data")
}

struct GetMapsReq {
    1: string id (api.path="id")
}

struct GetMapsResp {
    1: list<ReviewMapSummary> maps (api.body="maps")
    2: i64 total (api.body="total")
}

struct GetMapReq {
    1: string id (api.path="id")
    2: string map_id (api.path="mapId")
}

struct GetMapResp {
    1: string id (api.body="id")
    2: string review_id (api.body="review_id")
    3: string session_id (api.body="session_id")
    4: string title (api.body="title")
    5: string description (api.body="description")
    6: string sections (api.body="sections")
    7: string dependencies (api.body="dependencies")
    8: string summary (api.body="summary")
    9: i64 file_count (api.body="file_count")
    10: i64 section_count (api.body="section_count")
    11: string mermaid (api.body="mermaid")
    12: string created_at (api.body="created_at")
}

struct PreviewMapReq {
    1: string id (api.path="id")
    2: string diff (api.body="diff")
}

struct PreviewMapResp {
    1: string map_data (api.body="map_data")
}

struct GetSessionMapReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
}

struct GetSessionMapResp {
    1: string id (api.body="id")
    2: string review_id (api.body="review_id")
    3: string session_id (api.body="session_id")
    4: string title (api.body="title")
    5: string description (api.body="description")
    6: string sections (api.body="sections")
    7: string dependencies (api.body="dependencies")
    8: string summary (api.body="summary")
    9: i64 file_count (api.body="file_count")
    10: i64 section_count (api.body="section_count")
    11: string mermaid (api.body="mermaid")
    12: string created_at (api.body="created_at")
}
