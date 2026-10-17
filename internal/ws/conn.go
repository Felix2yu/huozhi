package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Conn 包装一个用户 WebSocket 连接
type Conn struct {
	hub    *Hub
	userID uint
	send   chan []byte
	conn   *websocket.Conn
}

func NewConn(hub *Hub, userID uint, ws *websocket.Conn) *Conn {
	return &Conn{
		hub:    hub,
		userID: userID,
		send:   make(chan []byte, 256),
		conn:   ws,
	}
}

// ReadPump 读取循环（处理心跳 pong）
func (c *Conn) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	// pong 到达即顺延读超时；顺延失败说明连接已经坏了，把错误交回去让
	// ReadMessage 尽快退出，别留一个读不到数据的死连接。
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// WritePump 写入循环（定时 ping）
func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				// 通道已关闭，发一帧 Close 后退出；发不出去也只能退出。
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			// 发送 ping 消息体让客户端知道服务器还活着
			ping, _ := json.Marshal(Message{Type: "ping", Timestamp: time.Now()})
			select {
			case c.send <- ping:
			default:
			}
		}
	}
}

// SafeLog 日志辅助
func SafeLog(format string, args ...any) {
	log.Printf(format, args...)
}
