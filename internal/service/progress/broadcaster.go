package progress

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"
)

type broadcaster struct {
	rooms      map[string]map[*websocket.Conn]bool
	mu         sync.RWMutex
	register   chan *registration
	unregister chan *registration
	broadcast  chan *broadcastMessage
	stop       chan struct{}
}

type registration struct {
	room string
	conn *websocket.Conn
}

type broadcastMessage struct {
	room    string
	message []byte
}

func newBroadcaster() *broadcaster {
	b := &broadcaster{
		rooms:      make(map[string]map[*websocket.Conn]bool),
		register:   make(chan *registration),
		unregister: make(chan *registration),
		broadcast:  make(chan *broadcastMessage),
		stop:       make(chan struct{}),
	}
	go b.run()
	return b
}

func (b *broadcaster) run() {
	for {
		select {
		case reg := <-b.register:
			b.handleRegister(reg)
		case reg := <-b.unregister:
			b.handleUnregister(reg)
		case msg := <-b.broadcast:
			b.handleBroadcast(msg)
		case <-b.stop:
			return
		}
	}
}

func (b *broadcaster) handleRegister(reg *registration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.rooms[reg.room]; !ok {
		b.rooms[reg.room] = make(map[*websocket.Conn]bool)
	}
	b.rooms[reg.room][reg.conn] = true
	hlog.Infof("websocket client connected to room: %s (total: %d)", reg.room, len(b.rooms[reg.room]))
}

func (b *broadcaster) handleUnregister(reg *registration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if conns, ok := b.rooms[reg.room]; ok {
		if _, ok := conns[reg.conn]; ok {
			delete(conns, reg.conn)
			_ = reg.conn.Close()
			hlog.Infof("websocket client disconnected from room: %s (total: %d)", reg.room, len(conns))
			if len(conns) == 0 {
				delete(b.rooms, reg.room)
			}
		}
	}
}

func (b *broadcaster) handleBroadcast(msg *broadcastMessage) {
	b.mu.RLock()
	conns, ok := b.rooms[msg.room]
	if !ok {
		b.mu.RUnlock()
		return
	}
	connections := make([]*websocket.Conn, 0, len(conns))
	for conn := range conns {
		connections = append(connections, conn)
	}
	b.mu.RUnlock()

	for _, conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msg.message); err != nil {
			hlog.Errorf("failed to send websocket message: %v", err)
			b.Unregister(msg.room, conn)
		}
	}
}

func (b *broadcaster) Register(room string, conn *websocket.Conn) {
	b.register <- &registration{room: room, conn: conn}
}

func (b *broadcaster) Unregister(room string, conn *websocket.Conn) {
	b.unregister <- &registration{room: room, conn: conn}
}

func (b *broadcaster) Broadcast(room string, message []byte) {
	b.broadcast <- &broadcastMessage{room: room, message: message}
}

func (b *broadcaster) BroadcastEvent(room string, event progressEvent) {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		hlog.Errorf("failed to marshal progress event: %v", err)
		return
	}
	b.Broadcast(room, data)
}

func (b *broadcaster) GetRoomCount(room string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if conns, ok := b.rooms[room]; ok {
		return len(conns)
	}
	return 0
}

func (b *broadcaster) GetTotalConnections() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := 0
	for _, conns := range b.rooms {
		total += len(conns)
	}
	return total
}

func (b *broadcaster) Close() {
	close(b.stop)
	b.mu.Lock()
	defer b.mu.Unlock()
	for room, conns := range b.rooms {
		for conn := range conns {
			_ = conn.Close()
		}
		delete(b.rooms, room)
	}
}
