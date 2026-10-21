# Performance Tuning Guide

## Overview

This guide provides recommendations for tuning the review-service performance in production environments.

## Cache Configuration

### LRU Cache

The service uses an in-memory LRU cache with TTL support for caching frequently accessed data.

**Default Configuration:**
- TTL: 5 minutes
- Max Items: 1000
- Cleanup Interval: 2.5 minutes (TTL/2)

**Tuning Recommendations:**

```go
// For high-traffic environments
cache := cache.NewLRUCache(
    10*time.Minute,  // Longer TTL for stable data
    5000,            // More items for better hit rate
)
```

**Monitoring Cache Performance:**

```go
stats := cache.Stats()
hitRate := stats.HitRate()

// Target: > 80% hit rate
if hitRate < 0.8 {
    // Consider increasing cache size or TTL
}
```

### Cache Hit Rate Optimization

| Hit Rate | Action |
|----------|--------|
| > 90% | Excellent - no changes needed |
| 80-90% | Good - monitor and adjust if needed |
| 70-80% | Consider increasing cache size |
| < 70% | Review caching strategy, increase TTL or size |

## Rate Limiting

### Global Rate Limiting

限流参数**配置驱动**，由 `internal/config` 的 `GetRateLimit()` 解析，未配置时回退默认值。

**Default Configuration:**
- Requests per second: 100
- Burst size: 200

**配置方式（环境变量）:**
```sh
RATE_LIMIT_RPS=500     # 每秒请求数
RATE_LIMIT_BURST=1000  # 突发容量
```

**应用位置:**
```go
// internal/modules/server.go
rps, burst := cfg.GetRateLimit()
h.Use(middleware.RateLimitMiddleware(rps, burst))
```

限流中间件按 **ClientIP 分桶**（KeyedLimiter，每 key 独立计数），实现见
`internal/middleware/ratelimit.go`（委托 ekit 的 `mw.RateLimitMiddleware`）。

> 早期版本文档提到过 `middleware.PerUserRateLimitMiddleware`——该中间件**不存在**，
> 当前只有上述按 IP 分桶的全局限流。

## Database Optimization

### Connection Pooling

连接池参数**配置驱动**，由 `DATABASE_MAX_OPEN` / `DATABASE_MAX_IDLE` 环境变量控制，
经 `gormx.WithConnectionPool` 应用。

```go
// internal/repository/postgres/db.go
db, err := gorm.Open(
    postgres.Open(dsn),
    &gorm.Config{},
    gormx.WithConnectionPool(cfg.MaxIdleConns, cfg.MaxOpenConns, 30*time.Minute),
)
if sqlDB, err := db.DB(); err == nil {
    // 空闲连接 5 分钟后回收，防止 DB 端 wait_timeout 静默关闭后取到失效连接
    sqlDB.SetConnMaxIdleTime(5 * time.Minute)
}
```

### Query Optimization

1. **Indexes（实际 schema）**

索引由 GORM tag 声明，随 AutoMigrate 建立。`reviews` 表当前有：

| 索引 | 列 | 用途 |
|---|---|---|
| `idx_provider_repo_status` | `(provider, repo, status)` | 按平台/仓库/状态过滤——最常用的列表查询 |
| `idx_reviews_pr` | `(pull_request_id)` | 按 PR 查评审 |
| `idx_reviews_mr` | `(merge_request_iid)` | 按 MR 查评审 |
| `idx`（单列） | `author` | 按作者过滤 |

`sessions` 表的 `review_id` 外键列同样带索引。

2. **Use Pagination:**
```go
// Instead of loading all records
db.Offset(offset).Limit(limit).Find(&reviews)
```

3. **Select Only Needed Fields:**
```go
db.Select("id", "review_id", "status").Find(&reviews)
```

## WebSocket Performance

### Connection Management

```go
// biz/handler/progress_service/progress_service.go
upgrader := websocket.HertzUpgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}
```

### Broadcasting Optimization

进度事件的出口是**单源总线**：`internal/service/progress/broadcaster.go` 桥接
底座 `agent-harness/eventbus` 与 WebSocket，事件经总线流向 broadcaster，
不存在多路直推。

1. **Room-Based Broadcasting:**
   - 连接按 review ID 注册到房间（`Register(room, conn)`）
   - 只向相关房间广播（`BroadcastEvent(room, event)`）
   - 房间 map 以 Conn 接口值（含指针身份）为 key，避免同值连接互相覆盖

2. **Single Publish Point:**
   - 进度事件统一经 `eventbus` 发布，tracker 不直接操作 WebSocket
   - 这保证订阅者（WebSocket 桥、未来的其他消费者）看到的事件顺序一致

## Memory Management

### Goroutine Leaks

Monitor goroutine count:
```go
import "runtime"

goroutines := runtime.NumGoroutine()
if goroutines > 1000 {
    // Investigate potential leaks
}
```

### Garbage Collection

Tune GC for low latency:
```go
import "runtime/debug"

debug.SetGCPercent(50)  // More frequent GC, lower memory usage
```

## Monitoring

### Key Metrics to Monitor

1. **Cache Hit Rate:** Target > 80%
2. **Request Latency:** Target < 100ms for reads
3. **Error Rate:** Target < 1%
4. **Goroutine Count:** Monitor for leaks
5. **Memory Usage:** Monitor for growth

### Prometheus Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "cache_hits_total",
        Help: "Total number of cache hits",
    })
    
    requestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "request_duration_seconds",
        Help:    "Request duration in seconds",
        Buckets: prometheus.DefBuckets,
    })
)
```

## Load Testing

### Using k6

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 100 },  // Ramp up
    { duration: '1m', target: 100 },   // Stay at 100 users
    { duration: '30s', target: 0 },    // Ramp down
  ],
};

export default function () {
  const res = http.get('http://localhost:8080/api/v1/reviews');
  check(res, { 'status was 200': (r) => r.status === 200 });
  sleep(1);
}
```

## Production Checklist

- [ ] Cache TTL and size configured appropriately
- [ ] Rate limiting enabled and tuned
- [ ] Database connection pool configured
- [ ] Indexes added for frequently queried fields
- [ ] WebSocket buffer sizes configured
- [ ] Monitoring and alerting set up
- [ ] Load testing performed
- [ ] Memory profiling done
- [ ] Goroutine leak detection enabled
