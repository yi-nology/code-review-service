package postgres

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/testutil"

	gormDB "gorm.io/gorm"
)

// testDB 是所有集成测试共享的、连到 testcontainer 的 gorm 句柄。
// 由 TestMain 在 docker 可用时启动；不可用时为 nil 且整包跳过。
var testDB *gormDB.DB

// reviewTables 是被 review/session 集成测试读写的表，测试间 TRUNCATE 隔离。
var reviewTables = []string{"reviews", "review_sessions"}

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, db, err := testutil.StartPostgres(ctx)
	if err != nil {
		// docker 不可用（如无 Docker 环境）时整包跳过，不报失败。
		// CI 有 docker 时正常跑，构成持久层回归网。
		log.Printf("postgres testcontainer unavailable, skipping DB integration tests: %v", err)
		os.Exit(0)
	}
	testDB = db

	code := m.Run()

	if container != nil {
		ctx2, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := container.Terminate(ctx2); err != nil {
			log.Printf("terminate postgres container: %v", err)
		}
	}
	os.Exit(code)
}

// resetTables 清空集成测试表，确保每个测试看到干净状态（ClaimQueued/RequeueStale 等
// 按状态跨行查询的方法对脏数据敏感）。
func resetTables(t *testing.T) {
	t.Helper()
	if err := truncateTables(t, reviewTables...); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// truncateTables 清空指定表，用于需要清理 reviewTables 之外表的测试。
func truncateTables(t *testing.T, tables ...string) error {
	t.Helper()
	if testDB == nil {
		t.Skip("testDB not initialized (docker unavailable)")
	}
	return testutil.Truncate(testDB, tables...)
}
