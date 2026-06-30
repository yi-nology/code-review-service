package progress

import "github.com/hertz-contrib/websocket"

// Broadcaster 抽象进度事件广播的契约。
//
// PR-12 修复：从返回 unexported *broadcaster 改为返回 Broadcaster 接口，
// 让 handler 测试可以通过 mock 实现注入。
// *broadcaster（broadcaster.go）自然实现本接口。
type Broadcaster interface {
	Register(room string, conn *websocket.Conn)
	Unregister(room string, conn *websocket.Conn)
	BroadcastEvent(room string, event progressEvent)
	Close()
}

// Compile-time assertion: *broadcaster 必须实现 Broadcaster 接口。
var _ Broadcaster = (*broadcaster)(nil)
