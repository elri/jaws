package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputText.WriteHtmlInput(e, w, "password")
}

func NewUiPassword(tags []interface{}, vp ValueProxy) (ui *UiPassword) {
	ui = &UiPassword{
		UiInputText: UiInputText{
			UiInput: UiInput{
				UiHtml:     UiHtml{Tags: tags},
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Password(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
