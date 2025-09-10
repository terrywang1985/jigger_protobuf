// ws_conn.go
package main

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type WSConnection struct {
	conn *websocket.Conn
}

func NewWSConnection(wsConn *websocket.Conn) *WSConnection {
	return &WSConnection{conn: wsConn}
}

func (w *WSConnection) Read(b []byte) (n int, err error) {
	_, message, err := w.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	copy(b, message)
	return len(message), nil
}

func (w *WSConnection) Write(b []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *WSConnection) Close() error {
	return w.conn.Close()
}

func (w *WSConnection) RemoteAddr() net.Addr {
	return w.conn.RemoteAddr()
}

func (w *WSConnection) SetDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

func (w *WSConnection) SetReadDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

func (w *WSConnection) SetWriteDeadline(t time.Time) error {
	return w.conn.SetWriteDeadline(t)
}
