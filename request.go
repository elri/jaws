package jaws

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/linkdata/deadlock"
)

// ConnectFn can be used to interact with a Request before message processing starts.
// Returning an error causes the Request to abort, and the WebSocket connection to close.
type ConnectFn func(rq *Request) error

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn func(rq *Request, id, evt, val string) error

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Note that we have to store the context inside the struct because there is no call chain
// between the Request being created and it being used once the WebSocket is created.
type Request struct {
	Jaws      *Jaws              // (read-only) the JaWS instance the Request belongs to
	JawsKey   uint64             // (read-only) a random number used in the WebSocket URI to identify this Request
	Created   time.Time          // (read-only) when the Request was created, used for automatic cleanup
	Initial   *http.Request      // (read-only) initial HTTP request passed to Jaws.NewRequest
	Context   context.Context    // (read-only) context passed to Jaws.NewRequest
	remoteIP  net.IP             // (read-only) remote IP, or nil
	session   *Session           // (read-only) session, if established
	sendCh    chan *Message      // (read-only) direct send message channel
	mu        deadlock.RWMutex   // protects following
	connectFn ConnectFn          // a ConnectFn to call before starting message processing for the Request
	elems     map[string]EventFn // map of registered HTML id's
}

type eventFnCall struct {
	fn  EventFn
	msg *Message
}

var metaIds = map[string]struct{}{
	" reload":   {},
	" ping":     {},
	" redirect": {},
	" alert":    {},
}

var requestPool = sync.Pool{New: func() interface{} {
	return &Request{
		elems:  make(map[string]EventFn),
		sendCh: make(chan *Message),
	}
}}

func newRequest(ctx context.Context, jw *Jaws, jawsKey uint64, hr *http.Request) (rq *Request) {
	rq = requestPool.Get().(*Request)
	rq.Jaws = jw
	rq.JawsKey = jawsKey
	rq.Created = time.Now()
	rq.Initial = hr
	rq.Context = ctx
	if hr != nil {
		rq.remoteIP = parseIP(hr.RemoteAddr)
		if sess := jw.getSessionLocked(getCookieSessionsIds(hr.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
}

func (rq *Request) JawsKeyString() string {
	jawsKey := uint64(0)
	if rq != nil {
		jawsKey = rq.JawsKey
	}
	return JawsKeyString(jawsKey)
}

func (rq *Request) String() string {
	return "Request<" + rq.JawsKeyString() + ">"
}

func (rq *Request) start(hr *http.Request) error {
	rq.mu.RLock()
	expectIP := rq.remoteIP
	rq.mu.RUnlock()
	var actualIP net.IP
	if hr != nil {
		actualIP = parseIP(hr.RemoteAddr)
	}
	if equalIP(expectIP, actualIP) {
		return nil
	}
	return fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), expectIP.String(), actualIP.String())
}

func (rq *Request) recycle() {
	rq.mu.Lock()
	rq.Jaws = nil
	rq.JawsKey = 0
	rq.connectFn = nil
	rq.Initial = nil
	rq.Context = nil
	rq.remoteIP = nil
	if sess := rq.session; sess != nil {
		rq.session = nil
		sess.delRequest(rq)
	}
	// this gets optimized to calling the 'runtime.mapclear' function
	// we don't expect this to improve speed, but it will lower GC load
	for k := range rq.elems {
		delete(rq.elems, k)
	}
	rq.mu.Unlock()
	requestPool.Put(rq)
}

// HeadHTML returns the HTML code needed to write in the HTML page's HEAD section.
func (rq *Request) HeadHTML() template.HTML {
	s := rq.Jaws.headPrefix + rq.JawsKeyString() + `";</script>`
	return template.HTML(s) // #nosec G203
}

// GetConnectFn returns the currently set ConnectFn. That function will be called before starting the WebSocket tunnel if not nil.
func (rq *Request) GetConnectFn() (fn ConnectFn) {
	rq.mu.RLock()
	fn = rq.connectFn
	rq.mu.RUnlock()
	return
}

// SetConnectFn sets ConnectFn. That function will be called before starting the WebSocket tunnel if not nil.
func (rq *Request) SetConnectFn(fn ConnectFn) {
	rq.mu.Lock()
	rq.connectFn = fn
	rq.mu.Unlock()
}

// Session returns the Request's Session, or nil.
func (rq *Request) Session() *Session {
	return rq.session
}

// Get is shorthand for `Session().Get()` and returns the session value associated with the key, or nil.
// It no session is associated with the Request, returns nil.
func (rq *Request) Get(key string) interface{} {
	return rq.Session().Get(key)
}

// Set is shorthand for `Session().Set()` and sets a session value to be associated with the key.
// If value is nil, the key is removed from the session.
// Does nothing if there is no session is associated with the Request.
func (rq *Request) Set(key string, val interface{}) {
	rq.Session().Set(key, val)
}

// Broadcast sends a broadcast to all Requests except the current one.
func (rq *Request) Broadcast(msg *Message) {
	msg.from = rq
	rq.Jaws.Broadcast(msg)
}

// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests except this one.
func (rq *Request) Trigger(id, val string) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "trigger",
		Data: val,
	})
}

// SetInner sends a jid and new inner HTML to all Requests except this one.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetInner(jid string, innerHtml string) {
	rq.Broadcast(&Message{
		Elem: jid,
		What: "inner",
		Data: innerHtml,
	})
}

// SetTextValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetTextValue(jid, val string) {
	rq.Broadcast(&Message{
		Elem: jid,
		What: "value",
		Data: val,
	})
}

// SetFloatValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetFloatValue(jid string, val float64) {
	rq.Broadcast(&Message{
		Elem: jid,
		What: "value",
		Data: strconv.FormatFloat(val, 'f', -1, 64),
	})
}

// SetBoolValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetBoolValue(jid string, val bool) {
	rq.Broadcast(&Message{
		Elem: jid,
		What: "value",
		Data: strconv.FormatBool(val),
	})
}

// SetDateValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetDateValue(jid string, val time.Time) {
	rq.Broadcast(&Message{
		Elem: jid,
		What: "value",
		Data: val.Format(ISO8601),
	})
}

func (rq *Request) getDoneCh(msg *Message) (<-chan struct{}, <-chan struct{}) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if rq.Jaws == nil {
		panic(fmt.Sprintf("Request.Send(%v): request is dead", msg))
	}
	return rq.Jaws.Done(), rq.Context.Done()
}

// Send a message to the current Request only.
// Returns true if the message was successfully sent.
func (rq *Request) Send(msg *Message) bool {
	jawsDoneCh, ctxDoneCh := rq.getDoneCh(msg)
	select {
	case <-jawsDoneCh:
	case <-ctxDoneCh:
	case rq.sendCh <- msg:
		return true
	}
	return false
}

// SetAttr sets an attribute on the HTML element(s) on the current Request only.
// If the value is an empty string, a value-less attribute will be added (such as "disabled").
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetAttr(jid, attr, val string) {
	rq.Send(&Message{
		Elem: jid,
		What: "sattr",
		Data: attr + "\n" + val,
	})
}

// RemoveAttr removes a given attribute from the HTML element(s) for the current Request only.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) RemoveAttr(jid, attr string) {
	rq.Send(&Message{
		Elem: jid,
		What: "rattr",
		Data: attr,
	})
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id 'jaws-alert'.
// The lvl argument should be one of Bootstraps alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The default JaWS javascript only supports Bootstrap.js dismissable alerts.
func (rq *Request) Alert(lvl, msg string) {
	rq.Send(&Message{
		Elem: " alert",
		What: lvl,
		Data: msg,
	})
}

// AlertError calls Alert if the given error is not nil.
func (rq *Request) AlertError(err error) {
	if err != nil {
		rq.Send(makeAlertDangerMessage(rq.Jaws.Log(err)))
	}
}

// Redirect requests the current Request to navigate to the given URL.
func (rq *Request) Redirect(url string) {
	rq.Send(&Message{
		Elem: " redirect",
		What: url,
	})
}

// RegisterEventFn records the given HTML 'jid' attribute as a valid target
// for dynamic updates using the given event function (which may be nil).
//
// If the jid argument is an empty string, a unique jid will be generated.
//
// If fn argument is nil, a pre-existing event function won't be overwritten.
//
// Returns the (possibly generated) jid.
func (rq *Request) RegisterEventFn(jid string, fn EventFn) string {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if jid != "" {
		if _, ok := rq.elems[jid]; ok {
			if fn == nil {
				return jid
			}
		}
		rq.elems[jid] = fn
	} else {
		for {
			jid = MakeID()
			if _, ok := rq.elems[jid]; !ok {
				rq.elems[jid] = fn
				break
			}
		}
	}
	return jid
}

// Register calls RegisterEventFn(id, nil).
// Useful in template constructs like:
//
//	<div jid="{{$.Register `foo`}}">
func (rq *Request) Register(jid string) string {
	return rq.RegisterEventFn(jid, nil)
}

// GetEventFn checks if a given HTML element is registered and returns
// the it's event function (or nil) along with a boolean indicating
// if it's a registered ID.
func (rq *Request) GetEventFn(jid string) (fn EventFn, ok bool) {
	rq.mu.RLock()
	if fn, ok = rq.elems[jid]; !ok {
		_, ok = metaIds[jid]
	}
	rq.mu.RUnlock()
	return
}

// SetEventFn sets the event function for the given jid to be the given function.
// Passing nil for the function is legal, and has the effect of ensuring the
// jid can be the target of DOM updates but not to send Javascript events.
// Note that you can only have one event function per jid.
func (rq *Request) SetEventFn(jid string, fn EventFn) {
	rq.mu.Lock()
	rq.elems[jid] = fn
	rq.mu.Unlock()
}

// OnEvent calls SetEventFn.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnEvent(jid string, fn EventFn) error {
	rq.SetEventFn(jid, fn)
	return nil
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan *Message, incomingMsgCh <-chan *Message, outboundMsgCh chan<- *Message) {
	jawsDoneCh := rq.Jaws.Done()
	ctxDoneCh := rq.Context.Done()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
		rq.Jaws.unsubscribe(broadcastMsgCh)
		close(eventCallCh)
		for {
			select {
			case <-eventCallCh:
			case <-rq.sendCh:
			case <-incomingMsgCh:
			case <-eventDoneCh:
				close(outboundMsgCh)
				return
			}
		}
	}()

	for {
		var msg *Message
		incoming := false

		select {
		case <-jawsDoneCh:
		case <-ctxDoneCh:
		case msg = <-rq.sendCh:
		case msg = <-broadcastMsgCh:
		case msg = <-incomingMsgCh:
			// messages incoming from the WebSocket are not to be resent out on
			// the WebSocket again, so note that this is an incoming message
			incoming = true
		}

		if msg == nil {
			// one of the channels are closed, so we're done
			return
		}

		if msg.from == rq {
			// don't process broadcasts that originate from ourselves
			continue
		}

		// only ever process messages for registered elements
		if fn, ok := rq.GetEventFn(msg.Elem); ok {
			// messages incoming from WebSocket or trigger messages
			// won't be sent out on the WebSocket, but will queue up a
			// call to the event function (if any)
			if incoming || msg.What == "trigger" {
				if fn != nil {
					select {
					case eventCallCh <- eventFnCall{fn: fn, msg: msg}:
					default:
						rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, msg))
						return
					}
				}
				continue
			}

			// "hook" messages are used to synchronously call an event function.
			// the function must not send any messages itself, but may return
			// an error to be sent out as an alert message.
			// primary usecase is tests.
			if msg.What == "hook" {
				msg = makeAlertDangerMessage(fn(rq, msg.Elem, msg.What, msg.Data))
			}

			if msg != nil {
				select {
				case <-jawsDoneCh:
				case <-ctxDoneCh:
				case outboundMsgCh <- msg:
				default:
					rq.Jaws.MustLog(fmt.Errorf("jaws: %v: outboundMsgCh is full sending %v", rq, msg))
					return
				}
			}
		}
	}
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- *Message, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		if err := call.fn(rq, call.msg.Elem, call.msg.What, call.msg.Data); err != nil {
			select {
			case outboundMsgCh <- makeAlertDangerMessage(err):
			default:
				_ = rq.Jaws.Log(fmt.Errorf("jaws: outboundMsgCh full sending event error '%s'", err.Error()))
			}
		}
	}
}

// onConnect calls the Request's ConnectFn if it's not nil, and returns the error from it.
// Returns nil if ConnectFn is nil.
func (rq *Request) onConnect() (err error) {
	rq.mu.RLock()
	connectFn := rq.connectFn
	rq.mu.RUnlock()
	if connectFn != nil {
		err = connectFn(rq)
	}
	return
}

func makeAlertDangerMessage(err error) (msg *Message) {
	if err != nil {
		msg = &Message{
			Elem: " alert",
			What: "danger",
			Data: html.EscapeString(err.Error()),
		}
	}
	return
}

// defaultChSize returns a reasonable buffer size for our data channels
func (rq *Request) defaultChSize() (n int) {
	rq.mu.RLock()
	n = 8 + len(rq.elems)*4
	rq.mu.RUnlock()
	return
}

func (rq *Request) maybeEvent(id, event string, fn ClickFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == event {
				err = fn(rq)
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

func (rq *Request) maybeClick(jid string, fn ClickFn) string {
	return rq.maybeEvent(jid, "click", fn)
}

func (rq *Request) maybeInputText(jid string, fn InputTextFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				err = fn(rq, val)
			}
			return
		}
	}
	return rq.RegisterEventFn(jid, wf)
}

func (rq *Request) maybeInputFloat(jid string, fn InputFloatFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v float64
				if val != "" {
					if v, err = strconv.ParseFloat(val, 64); err != nil {
						return
					}
				}
				err = fn(rq, v)
			}
			return
		}
	}
	return rq.RegisterEventFn(jid, wf)
}

func (rq *Request) maybeInputBool(jid string, fn InputBoolFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v bool
				if val != "" {
					if v, err = strconv.ParseBool(val); err != nil {
						return
					}
				}
				err = fn(rq, v)
			}
			return
		}
	}
	return rq.RegisterEventFn(jid, wf)
}

func (rq *Request) maybeInputDate(jid string, fn InputDateFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v time.Time
				if val != "" {
					if v, err = time.Parse(ISO8601, val); err != nil {
						return
					}
				}
				err = fn(rq, v)
			}
			return
		}
	}
	return rq.RegisterEventFn(jid, wf)
}
