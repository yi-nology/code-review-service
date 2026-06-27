package progress

import (
	"encoding/json"
	"time"

	"github.com/oekit/network/wsutil"
	"github.com/oekit/logx"
	"github.com/hertz-contrib/websocket"
)

// wsConnAdapter 适配 websocket.Conn 到 wsutil.Conn 接口。
type wsConnAdapter struct {
	conn *websocket.Conn
}

func (a *wsConnAdapter) WriteMessage(messageType int, data []byte) error {
	return a.conn.WriteMessage(messageType, data)
}

func (a *wsConnAdapter) Close() error {
	return a.conn.Close()
}

type broadcaster struct {
	bc *wsutil.Broadcaster
}

func newBroadcaster() *broadcaster {
	return &broadcaster{bc: wsutil.NewBroadcaster()}
}

func (b *broadcaster) Register(room string, conn *websocket.Conn) {
	b.bc.Register(room, &wsConnAdapter{conn: conn})
}

func (b *broadcaster) Unregister(room string, conn *websocket.Conn) {
	b.bc.Unregister(room, &wsConnAdapter{conn: conn})
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
