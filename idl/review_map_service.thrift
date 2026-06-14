include "review_map.thrift"

service reviewMapService {
    review_map.GenerateMapResp GenerateMap(1: review_map.GenerateMapReq req) (api.post="/api/v1/reviews/:id/maps")
    review_map.GetMapsResp GetMaps(1: review_map.GetMapsReq req) (api.get="/api/v1/reviews/:id/maps")
    review_map.GetMapResp GetMap(1: review_map.GetMapReq req) (api.get="/api/v1/reviews/:id/maps/:mapId")
    review_map.PreviewMapResp PreviewMap(1: review_map.PreviewMapReq req) (api.post="/api/v1/reviews/:id/maps/preview")
    review_map.GetSessionMapResp GetSessionMap(1: review_map.GetSessionMapReq req) (api.get="/api/v1/reviews/:id/sessions/:sessionId/map")
}
