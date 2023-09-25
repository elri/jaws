package jaws

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"strconv"
	"sync/atomic"
)

// An Element is an instance of a *Request, an UI object and a Jid.
type Element struct {
	ui       UI   // (read-only) the UI object
	jid      Jid  // (read-only) JaWS ID, unique to this Element within it's Request
	updating bool // about to have Update() called
	*Request      // (read-only) the Request the Element belongs to
}

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.ui, e.jid, e.Request.TagsOf(e))
}

// Tag adds the given tags to the Element.
func (e *Element) Tag(tags ...interface{}) {
	e.Request.Tag(e, tags...)
}

// HasTag returns true if this Element has the given tag.
func (e *Element) HasTag(tag interface{}) bool {
	return e.Request.HasTag(e, tag)
}

// Jid returns the JaWS ID for this Element, unique within it's Request.
func (e *Element) Jid() Jid {
	return e.jid
}

// UI returns the UI object.
func (e *Element) UI() UI {
	return e.ui
}

// Dirty marks this Element (only) as needing UI().JawsUpdate() to be called.
func (e *Element) Dirty() {
	if e != nil {
		e.Request.appendDirtyTags(e)
	}
}

// Render calls UI().JawsRender() for this Element.
func (e *Element) Render(w io.Writer, params []interface{}) {
	e.ui.JawsRender(e, w, params)
}

func (e *Element) ToHtml(val interface{}) template.HTML {
	var s string
	switch v := val.(type) {
	case string:
		s = v
	case template.HTML:
		return v
	case *atomic.Value:
		return e.ToHtml(v.Load())
	case fmt.Stringer:
		s = v.String()
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		s = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		s = strconv.Itoa(v)
	default:
		panic(fmt.Errorf("jaws: don't know how to render %T as template.HTML", v))
	}
	return template.HTML(html.EscapeString(s))
}
