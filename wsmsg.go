package jaws

import (
	"bytes"
	"fmt"
	"html"
	"strconv"

	"github.com/linkdata/jaws/what"
)

// wsMsg is a message sent to or from a WebSocket.
type wsMsg struct {
	Data string    // data to send
	Jid  Jid       // Jid to send, or negative to not send
	What what.What // command
}

func (m *wsMsg) IsValid() bool {
	return m.What.IsValid()
}

func (m *wsMsg) Append(b []byte) []byte {
	b = append(b, m.What.String()...)
	b = append(b, '\t')
	if m.Jid >= 0 {
		if m.Jid > 0 {
			b = m.Jid.Append(b)
		}
		b = append(b, '\t')
	}
	if len(m.Data) > 0 {
		b = strconv.AppendQuote(b, m.Data)
	}
	b = append(b, '\n')
	return b
}

func (m *wsMsg) Format() string {
	return string(m.Append(nil))
}

// wsParse parses an incoming text buffer into a message.
func wsParse(txt []byte) (wsMsg, bool) {
	txt = bytes.ToValidUTF8(txt, nil) // we don't trust client browsers
	if len(txt) > 0 && txt[len(txt)-1] == '\n' {
		if nl1 := bytes.IndexByte(txt, '\t'); nl1 >= 0 {
			if nl2 := bytes.IndexByte(txt[nl1+1:], '\t'); nl2 >= 0 {
				nl2 += nl1 + 1
				// What       ... Jid              ... Data                  ... EOL
				// txt[0:nl1] ... txt[nl1+1 : nl2] ... txt[nl2+1:len(txt)-1] ... \n
				if wht := what.Parse(string(txt[0:nl1])); wht.IsValid() {
					data := string(txt[nl2+1 : len(txt)-1])
					if txt[nl2+1] == '"' {
						var err error
						if data, err = strconv.Unquote(data); err != nil {
							return wsMsg{}, false
						}
					}
					return wsMsg{
						Data: data,
						Jid:  JidParseString(string(txt[nl1+1 : nl2])),
						What: wht,
					}, true
				}
			}
		}
	}
	return wsMsg{}, false
}

func (m *wsMsg) String() string {
	return fmt.Sprintf("wsMsg{%s, %d, %q}", m.What, m.Jid, m.Data)
}

func (m *wsMsg) FillAlert(err error) {
	m.Jid = 0
	m.What = what.Alert
	m.Data = "danger\n" + html.EscapeString(err.Error())
}
