package jaws

import (
	"html/template"
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.UiInputFloat.WriteHtmlInput(e, w, "number", params...)
}

func NewUiNumber(vp ValueProxy) (ui *UiNumber) {
	ui = &UiNumber{
		UiInputFloat: UiInputFloat{
			UiInput: NewUiInput(vp),
		},
	}
	return
}

func (rq *Request) Number(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiNumber(MakeValueProxy(value)), params...)
}
