package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Message 推送消息格式
type Message struct {
	Type      string          `json:"type"`                // sync / ping / alert / error
	Table     string          `json:"table,omitempty"`     // transactions / accounts / budgets / ...
	Action    string          `json:"action,omitempty"`    // create / update / delete / over_budget
	ID        uint            `json:"id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`      // 附加数据（如超支详情）
	Version   int64           `json:"version,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// Hub 管理所有 WebSocket 连接，按 user_id 分组
type Hub struct {
	mu       sync.RWMutex
	clients  map[uint]map[*Conn]bool // user_id -> set of *Conn
	broadcast chan *Event
	register  chan *Conn
	unregister chan *Conn
}

// Event 内部广播事件
type Event struct {
	UserID  uint
	Message Message
}

var DefaultHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uint]map[*Conn]bool),
		broadcast:  make(chan *Event, 256),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[c.userID]; !ok {
				h.clients[c.userID] = make(map[*Conn]bool)
			}
			h.clients[c.userID][c] = true
			h.mu.Unlock()
			log.Printf("[WS] conn+ user=%d total=%d", c.userID, h.Count())

		case c := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.clients[c.userID]; ok {
				delete(conns, c)
				if len(conns) == 0 {
					delete(h.clients, c.userID)
				}
			}
			h.mu.Unlock()
			close(c.send)
			log.Printf("[WS] conn- user=%d total=%d", c.userID, h.Count())

		case ev := <-h.broadcast:
			msg, _ := json.Marshal(ev.Message)
			h.mu.RLock()
			for c := range h.clients[ev.UserID] {
				select {
				case c.send <- msg:
				default:
					// 发送缓冲区满，跳过（避免阻塞广播循环）
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Count 总连接数
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, m := range h.clients {
		n += len(m)
	}
	return n
}

// Broadcast 向指定用户的所有连接推送一条同步消息
func (h *Hub) Broadcast(userID uint, table, action string, id uint) {
	h.broadcast <- &Event{
		UserID: userID,
		Message: Message{
			Type:      "sync",
			Table:     table,
			Action:    action,
			ID:        id,
			Timestamp: time.Now(),
		},
	}
}

// BroadcastWithData 推送带附加数据的消息（如超支提醒）
func (h *Hub) BroadcastWithData(userID uint, msg Message) {
	msg.Timestamp = time.Now()
	if msg.Type == "" {
		msg.Type = "sync"
	}
	h.broadcast <- &Event{UserID: userID, Message: msg}
}
