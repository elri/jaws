package jaws

import (
	"context"
	"net/http"

	"nhooyr.io/websocket"
)

// ServeHTTP implements http.HanderFunc.
//
// Assumes UseRequest() have been successfully called for the Request.
func (rq *Request) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, nil)
	if err == nil {
		if err = rq.onConnect(); err == nil {
			incomingMsgCh := make(chan wsMsg)
			broadcastMsgCh := rq.Jaws.subscribe(rq, 1)
			outboundCh := make(chan string, cap(broadcastMsgCh))
			go wsReader(rq.ctx, rq.cancelFn, rq.Jaws.Done(), incomingMsgCh, ws) // closes incomingMsgCh
			go wsWriter(rq.ctx, rq.cancelFn, rq.Jaws.Done(), outboundCh, ws)    // calls ws.Close()
			rq.process(broadcastMsgCh, incomingMsgCh, outboundCh)               // unsubscribes broadcastMsgCh, closes outboundMsgCh
		} else {
			defer ws.Close(websocket.StatusNormalClosure, err.Error())
			var msg wsMsg
			msg.FillAlert(rq.Jaws.Log(err))
			_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
		}
	}
	if err != nil {
		rq.cancel(err)
		_ = rq.Jaws.Log(err)
	}
	rq.recycle()
}

// wsReader reads websocket text messages, parses them and sends them on incomingMsgCh.
//
// Closes incomingMsgCh on exit.
func wsReader(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, incomingMsgCh chan<- wsMsg, ws *websocket.Conn) {
	var typ websocket.MessageType
	var txt []byte
	var err error
	defer close(incomingMsgCh)
	for err == nil {
		if typ, txt, err = ws.Read(ctx); typ == websocket.MessageText {
			if msg, ok := wsParse(txt); ok {
				select {
				case <-ctx.Done():
					return
				case <-jawsDoneCh:
					return
				case incomingMsgCh <- msg:
				}
			}
		}
	}
	if ccf != nil {
		ccf(err)
	}
}

// wsWriter reads JaWS messages from outboundMsgCh, formats them and writes them to the websocket.
//
// Closes the websocket on exit.
func wsWriter(ctx context.Context, ccf context.CancelCauseFunc, jawsDoneCh <-chan struct{}, outboundCh <-chan string, ws *websocket.Conn) {
	defer ws.Close(websocket.StatusNormalClosure, "")
	var err error
	for err == nil {
		select {
		case <-ctx.Done():
			return
		case <-jawsDoneCh:
			return
		case msg, ok := <-outboundCh:
			if !ok {
				return
			}
			err = ws.Write(ctx, websocket.MessageText, []byte(msg))
		}
	}
	if ccf != nil {
		ccf(err)
	}
}
