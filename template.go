package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/what"
)

type Template struct {
	Dot interface{}
	*template.Template
}

// GetTemplate resolves 'v' to a *template.Template or panics.
func (rq *Request) MustTemplate(v interface{}) (tp *template.Template) {
	switch v := v.(type) {
	case *template.Template:
		tp = v
	case string:
		tp = rq.Jaws.Template.Lookup(v)
	}
	if tp == nil {
		panic(fmt.Errorf("expected template, not %v", v))
	}
	return
}

func (rq *Request) MakeTemplate(templ, dot interface{}) Template {
	return Template{Template: rq.MustTemplate(templ), Dot: dot}
}

func (t Template) String() string {
	return fmt.Sprintf("{%q, %s}", t.Template.Name(), TagString(t.Dot))
}

var _ UI = (*Template)(nil) // statically ensure interface is defined

func (t Template) JawsRender(e *Element, w io.Writer, params []interface{}) {
	e.Tag(t)
	attrs := parseParams(e, params, nil, nil, nil)
	writeUiDebug(e, w)
	maybePanic(t.Execute(w, With{Element: e, Dot: t.Dot, Attrs: strings.Join(attrs, " ")}))
}

func (t Template) JawsUpdate(e *Element) {
	var b bytes.Buffer
	e.Render(&b, nil)
	e.Replace(template.HTML(b.String()))
}

var _ EventHandler = (*Template)(nil) // statically ensure interface is defined

func (t Template) JawsEvent(e *Element, wht what.What, val string) error {
	if wht == what.Click {
		if h, ok := t.Dot.(ClickHandler); ok {
			return h.JawsClick(e, val)
		}
	}
	if h, ok := t.Dot.(EventHandler); ok {
		return h.JawsEvent(e, wht, val)
	}
	return nil
}
