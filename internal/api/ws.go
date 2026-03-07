package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	defer conn.CloseNow()

	ch := make(chan []byte, 64)

	s.wsMu.Lock()
	s.wsClients[ch] = struct{}{}
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, ch)
		s.wsMu.Unlock()
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case data := <-ch:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
