package state_machine

import (
	. "gopkg.in/check.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

var (
	url = "127.0.0.1:59232"
	job = "raft-instance"
)

func (t *T) TestShouldPush(c *C) {
	// given
	sm := NewPromMetricSm(url, job)

	// when
	res := sm.Exec("42")

	// then
	c.Assert(res, Equals, "Cmd 42 Applied.")
}

func (t *T) TestNoneIntegerCmd(c *C) {
	// given
	sm := NewPromMetricSm(url, job)

	// when
	res := sm.Exec("wrong-cmd")

	// then
	c.Assert(res, Equals, "Cmd Should Be Number Format Error: strconv.Atoi: parsing \"wrong-cmd\": invalid syntax")
}
