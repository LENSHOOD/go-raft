//go:build chaos
// +build chaos

package state_machine

import (
	. "gopkg.in/check.v1"
	"gopkg.in/resty.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

func (t *T) TestShouldPush(c *C) {
	// given
	sm := newPromMetricSm(url, job)
	gaugeStr := "42"

	// when
	res := sm.Exec(gaugeStr)

	// then
	c.Assert(res, Equals, "Cmd "+gaugeStr+" Applied.")

	// then stored
	metrics := map[string]interface{}{}
	_, err := resty.R().SetResult(&metrics).Get("http://" + url + "/api/v1/metrics")
	if err != nil {
		c.Logf("call push gateway error: %v", err)
		c.Fail()
	}

	dataArr := metrics["data"].([]interface{})
	for _, val := range dataArr {
		inner := val.(map[string]interface{})
		if v, exist := inner["raft_cmd_applied"]; exist {
			c.Assert(gaugeStr, Equals, v.(map[string]interface{})["metrics"].([]interface{})[0].(map[string]interface{})["value"].(string))
			return
		}
	}

	c.Logf("not found expected gauge value: %s, from raft_cmd_applied", gaugeStr)
	c.Fail()
}

func (t *T) TestNoneIntegerCmd(c *C) {
	// given
	sm := newPromMetricSm(url, job)

	// when
	res := sm.Exec("wrong-cmd")

	// then
	c.Assert(res, Equals, "Cmd Should Be Number Format Error: strconv.Atoi: parsing \"wrong-cmd\": invalid syntax")
}
