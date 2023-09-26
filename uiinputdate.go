package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiHtml
	TimeGetter
}

func (ui *UiInputDate) value(e *Element) string {
	return ui.JawsGetTime(e).Format(ISO8601)
}

func (ui *UiInputDate) WriteHtmlInput(e *Element, w io.Writer, jid Jid, htmltype string, params ...interface{}) {
	if tagger, ok := ui.TimeGetter.(TagGetter); ok {
		e.Tag(tagger.JawsGetTag(e))
	} else {
		e.Tag(ui.TimeGetter)
	}
	attrs := ui.parseParams(e, params)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, ui.value(e), attrs...))
}

func (ui *UiInputDate) JawsUpdate(u Updater) {
	u.SetValue(ui.value(u.Element))
}

func (ui *UiInputDate) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		err = ui.TimeGetter.(TimeSetter).JawsSetTime(e, v)
		e.Jaws.Dirty(ui.TimeGetter)
	}
	return
}
