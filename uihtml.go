package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	ClickHandler ClickHandler
	EventHandler EventHandler
	EventFn      EventFn // legacy
	Tag          any
}

func writeUiDebug(e *Element, w io.Writer) {
	if deadlock.Debug {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", e.jid, e.ui)
		for i, tag := range e.Request.TagsOf(e) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(TagString(tag))
		}
		sb.WriteByte(']')
		_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
	}
}

func (ui *UiHtml) parseGetter(e *Element, getter interface{}) {
	if getter != nil {
		if tagger, ok := getter.(TagGetter); ok {
			ui.Tag = tagger.JawsGetTag(e.Request)
		} else {
			ui.Tag = getter
		}
		e.Tag(ui.Tag)
		if ch, ok := getter.(ClickHandler); ok {
			ui.ClickHandler = ch
		}
		if eh, ok := getter.(EventHandler); ok {
			ui.EventHandler = eh
		}
	}
}

func parseParams(elem *Element, params []interface{}, ch *ClickHandler, eh *EventHandler, ef *EventFn) (attrs []string) {
	for i := range params {
		switch data := params[i].(type) {
		case template.HTML:
			attrs = append(attrs, string(data))
		case []template.HTML:
			for _, s := range data {
				attrs = append(attrs, string(s))
			}
		case string:
			attrs = append(attrs, data)
		case []string:
			attrs = append(attrs, data...)
		case EventFn:
			if ef != nil {
				*ef = data
			}
		case func(*Request, string) error: // ClickFn
			if data != nil && ef != nil {
				*ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Click {
						err = data(rq, jid)
					}
					return
				}
			}
		case func(*Request, string, string) error: // InputTextFn
			if data != nil && ef != nil {
				*ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						err = data(rq, jid, val)
					}
					return
				}
			}
		case func(*Request, string, bool) error: // InputBoolFn
			if data != nil && ef != nil {
				*ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v bool
						if val != "" {
							if v, err = strconv.ParseBool(val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, float64) error: // InputFloatFn
			if data != nil && ef != nil {
				*ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v float64
						if val != "" {
							if v, err = strconv.ParseFloat(val, 64); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, time.Time) error: // InputDateFn
			if data != nil && ef != nil {
				*ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v time.Time
						if val != "" {
							if v, err = time.Parse(ISO8601, val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		default:
			if ch != nil {
				if h, ok := data.(ClickHandler); ok {
					*ch = h
				}
			}
			if eh != nil {
				if h, ok := data.(EventHandler); ok {
					*eh = h
				}
			}
			elem.Tag(data)
		}
	}
	return
}

func (ui *UiHtml) parseParams(elem *Element, params []interface{}) (attrs []string) {
	attrs = parseParams(elem, params, &ui.ClickHandler, &ui.EventHandler, &ui.EventFn)
	return
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []interface{}) {
	panic(fmt.Errorf("jaws: UiHtml.JawsRender(%v) called", e))
}

func (ui *UiHtml) JawsUpdate(e *Element) {
	switch v := ui.Tag.(type) {
	case *NamedBoolArray:
		e.SetValue(v.Get())
	case StringGetter:
		e.SetValue(v.JawsGetString(e))
	case FloatGetter:
		e.SetValue(string(fmt.Append(nil, v.JawsGetFloat(e))))
	case BoolGetter:
		if v.JawsGetBool(e) {
			e.SetAttr("checked", "")
		} else {
			e.RemoveAttr("checked")
		}
	case TimeGetter:
		e.SetValue(v.JawsGetTime(e).Format(ISO8601))
	case HtmlGetter:
		e.SetInner(v.JawsGetHtml(e))
	case UI:
		v.JawsUpdate(e)
	case Tag:
		// do nothing
	default:
		panic(fmt.Errorf("jaws: UiHtml.JawsUpdate(%v): unhandled type: %T", e, v))
	}
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil { // LEGACY
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Click && ui.ClickHandler != nil {
		return ui.ClickHandler.JawsClick(e, val)
	}
	if ui.EventHandler != nil {
		return ui.EventHandler.JawsEvent(e, wht, val)
	}
	if wht == what.Input {
		switch data := ui.Tag.(type) {
		case *NamedBoolArray:
			data.Set(val, true)
		case StringSetter:
			err = data.JawsSetString(e, val)
		case FloatSetter:
			var v float64
			if val != "" {
				if v, err = strconv.ParseFloat(val, 64); err != nil {
					return
				}
			}
			err = data.JawsSetFloat(e, v)
		case BoolSetter:
			var v bool
			if val != "" {
				if v, err = strconv.ParseBool(val); err != nil {
					return
				}
			}
			err = data.JawsSetBool(e, v)
		case TimeSetter:
			var v time.Time
			if val != "" {
				if v, err = time.Parse(ISO8601, val); err != nil {
					return
				}
			}
			err = data.JawsSetTime(e, v)
		default:
			if deadlock.Debug {
				_ = e.Jaws.Log(fmt.Errorf("jaws: UiHtml.JawsEvent(%v, %s, %q): unhandled type: %T", e, wht, val, data))
			}
		}
		e.Dirty(ui.Tag)
	}
	return
}
