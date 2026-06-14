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
    1: string maps (api.body="maps")
    2: i64 total (api.body="total")
}

struct GetMapReq {
    1: string id (api.path="id")
    2: string map_id (api.path="mapId")
}

struct GetMapResp {
    1: string map_data (api.body="map_data")
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
    1: string map_data (api.body="map_data")
}
