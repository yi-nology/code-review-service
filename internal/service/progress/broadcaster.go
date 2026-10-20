package progress

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hertz-contrib/websocket"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/network/wsutil"
	"github.com/yi-nology/agent-harness/eventbus"
)

// bus.go：评审事件的**单源总线**——底座 agent-harness/eventbus。
//
// 此前进度事件只有一条出口：tracker 直接调 broadcaster.BroadcastEvent 推给 websocket。
// 把底座总线放在 tracker 与 websocket 之间，事件就有了单源发布点与多消费面
// （websocket 只是其中一个订阅者）——编排层想发事件不必知道前端的存在。
//
// 有损是有意的取舍：底座总线每订阅者 64 帧缓冲，满了丢帧但**发布永不阻塞**。
// 进度事件丢一帧不影响评审结论，阻塞发布却会拖慢主流程——取舍方向必须写明，
// 否则下一个看到「偶发丢事件」的人会以为是 bug。
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

// eventBridge 把底座总线上的帧转成前端订阅的进度事件格式并推给 websocket 客户端。
//
// 桥接方向只有「总线 → 前端」一条：编排侧不反向读总线。
type eventBridge struct {
	bus         *eventbus.Bus
	broadcaster *broadcaster
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// newEventBridge 启动桥接协程。ctx 取消即停止。
func newEventBridge(ctx context.Context, bus *eventbus.Bus, bc *broadcaster) *eventBridge {
	ctx, cancel := context.WithCancel(ctx)
	bridge := &eventBridge{bus: bus, broadcaster: bc, cancel: cancel}
	frames, unsub := bus.Subscribe(ctx)
	// 取消 ctx 即退订；这里显式调用一次，保证 ctx 之外的退订路径也被覆盖。
	defer func() { _ = unsub }()
	bridge.wg.Add(1)
	// GoSafe：转发路径 panic 不该带走整个进程（进度事件是副产物）。
	async.GoSafe(func() {
		defer bridge.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-frames:
				if !ok {
					return
				}
				bridge.forward(ev)
			}
		}
	})
	return bridge
}

// forward 转发一帧总线事件到对应评审房间。
func (b *eventBridge) forward(ev eventbus.Event) {
	raw, ok := ev.Payload[progressEventKey]
	if !ok {
		return
	}
	event, ok := raw.(progressEvent)
	if !ok {
		return
	}
	b.broadcaster.BroadcastEvent(roomOf(ev.CaseID), event)
}

// Close 停止桥接并等待退出。
func (b *eventBridge) Close() {
	b.cancel()
	b.wg.Wait()
}

// progressEventKey 是进度事件在总线载荷里的键名。
const progressEventKey = "progress"

// roomOf 由评审 ID 推出 websocket 房间名。
func roomOf(reviewID string) string { return "review:" + reviewID }
