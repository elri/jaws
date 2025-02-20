package jaws

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Text(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ss := newTestSetter("foo")
	want := `<input id="Jid.1" type="text" value="foo">`
	rq.Text(ss)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Text() = %q, want %q", got, want)
	}
	rq.inCh <- wsMsg{Data: "bar", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case <-ss.setCalled:
	}
	if ss.Get() != "bar" {
		t.Error(ss.Get())
	}
	select {
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}
	ss.Set("quux")
	rq.Dirty(ss)
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		if s != "Value\tJid.1\t\"quux\"\n" {
			t.Error("wrong Value")
		}
	}
	if ss.Get() != "quux" {
		t.Error("not quux")
	}
	if ss.SetCount() != 1 {
		t.Error("SetCount", ss.SetCount())
	}

	ss.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: "omg", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	if ss.Get() != "quux" {
		t.Error("unexpected change", ss.Get())
	}
}
