package jaws

import (
	"bytes"
	"html/template"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UI interface {
	JawsTags(rq *Request) (tags []interface{})
	JawsRender(e *Element, w io.Writer) (err error)
	JawsEvent(e *Element, wht what.What, val string) (err error)
}

func (rq *Request) newElementLocked(tags []interface{}, ui UI, data []interface{}) (elem *Element) {
	if len(tags) > 0 {
		elem = &Element{jid: len(rq.elems) + 1, ui: ui, Data: data, rq: rq}
		rq.elems = append(rq.elems, elem)
		jid := elem.Jid()
		rq.tagMap[jid] = append(rq.tagMap[jid], elem)
		for _, tag := range tags {
			rq.tagMap[tag] = append(rq.tagMap[tag], elem)
		}
	}
	return
}

func (rq *Request) GetElement(jid string) (e *Element) {
	if n, err := strconv.Atoi(jid); err == nil && n > 0 {
		rq.mu.RLock()
		if n <= len(rq.elems) {
			e = rq.elems[n-1]
		}
		rq.mu.RUnlock()
	}
	return
}

func (rq *Request) UI(ui UI, data ...interface{}) template.HTML {
	tags := ui.JawsTags(rq)
	rq.mu.Lock()
	elem := rq.newElementLocked(tags, ui, data)
	rq.mu.Unlock()
	var b bytes.Buffer
	if err := ui.JawsRender(elem, &b); err != nil {
		rq.Jaws.MustLog(err)
	}
	return template.HTML(b.String())
}

func (rq *Request) Render(tags []interface{}) {
	var todo []*Element
	rq.mu.RLock()
	for _, tag := range tags {
		todo = append(todo, rq.tagMap[tag]...)
	}
	rq.mu.RUnlock()
	var b bytes.Buffer
	for _, elem := range todo {
		if err := elem.UI().JawsRender(elem, &b); err != nil {
			rq.Jaws.MustLog(err)
		}
		b.Reset()
	}
}

type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}

func (rq *Request) Ui(elem Ui, attrs ...string) template.HTML {
	return elem.JawsUi(rq, attrs...)
}
