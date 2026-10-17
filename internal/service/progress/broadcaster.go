package progress

import (
	"encoding/json"
	"time"

	"github.com/oekit/network/network/wsutil"
	"github.com/oekit/logx"
	"github.com/hertz-contrib/websocket"
)

type broadcaster struct {
	bc *wsutil.Broadcaster
}

func newBroadcaster() *broadcaster {
	return &broadcaster{bc: wsutil.NewBroadcaster()}
}

// Register/Unregister 必须传入同一个 *websocket.Conn 实例：
// wsutil.Broadcaster 以 Conn 接口值（含指针身份）作为房间 map 的 key。
// 此前每次都 new 一个 wsConnAdapter，导致 Unregister 的 key 指针与 Register 时不同，
// 查找永远 miss，死连接泄漏。*websocket.Conn 本身即满足 wsutil.Conn 接口，
// 直接透传可保证 Register/Unregister 使用同一接口值。
func (b *broadcaster) Register(room string, conn *websocket.Conn) {
	b.bc.Register(room, conn)
}

func (b *broadcaster) Unregister(room string, conn *websocket.Conn) {
	b.bc.Unregister(room, conn)
}

func (b *broadcaster) BroadcastEvent(room string, event progressEvent) {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		logx.Errorf("failed to marshal progress event: %v", err)
		return
	}
	b.bc.Broadcast(room, data)
}

func (b *broadcaster) Close() {
	b.bc.Close()
}
