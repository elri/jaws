package jaws

import (
	"html/template"
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "span", "", params...)
}

func NewUiSpan(innerHtml Getter) *UiSpan {
	return &UiSpan{
		UiHtmlInner{
			UiGetter{
				Getter: innerHtml,
			},
		},
	}
}

func (rq *Request) Span(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiSpan(MakeValueProxy(innerHtml)), params...)
}
