package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "button", "button", params...)
}

func NewUiButton(innerHtml Getter) *UiButton {
	return &UiButton{
		UiHtmlInner{
			UiGetter{
				Getter: innerHtml,
			},
		},
	}
}

func (rq *Request) Button(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiButton(MakeValueProxy(innerHtml)), params...)
}
