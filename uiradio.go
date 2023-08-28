package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputBool.WriteHtmlInput(e, w, "radio", append(e.Data, `id="jid.`+e.Jid().String()+`"`))
}

func NewUiRadio(up Params) (ui *UiRadio) {
	ui = &UiRadio{
		UiInputBool: UiInputBool{
			UiInput:   NewUiInput(up),
			NamedBool: up.nb,
		},
	}
	return
}

func (rq *Request) Radio(params ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(NewParams(params)), params...)
}
