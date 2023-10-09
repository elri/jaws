package jaws

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type UiTextarea struct {
	UiHtml
	StringGetter
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.StringGetter)
	attrs := ui.parseParams(e, params)
	maybePanic(WriteHtmlInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGetString(e)), attrs...))
}

func (ui *UiTextarea) JawsUpdate(e *Element) {
	e.SetInner(template.HTML(ui.JawsGetString(e)))
}

func (ui *UiTextarea) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if wht == what.Input {
		err = ui.StringGetter.(StringSetter).JawsSetString(e, val)
		e.Dirty(ui.Tag)
	}
	return
}

func NewUiTextarea(g StringGetter) (ui *UiTextarea) {
	return &UiTextarea{
		StringGetter: g,
	}
}

func (rq *Request) Textarea(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTextarea(makeStringGetter(value)), params...)
}
