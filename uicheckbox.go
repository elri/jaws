package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputBool.WriteHtmlInput(rq, w, "checkbox", jid, data...)
}

func (rq *Request) Checkbox(tagstring string, val interface{}, attrs ...interface{}) template.HTML {
	ui := &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
		},
	}
	ui.ProcessValue(val)
	return rq.UI(ui, attrs...)
}
