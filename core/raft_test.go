package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestReplaceConfigWithinMe(c *C) {
	// given
	raw := commCfg.cluster
	// add two node and remove one node
	newMember := []Id{-11203, 190152, 96775, 2344359, 99811, 56867}

	// when
	raw.replaceTo(newMember)

	// then
	c.Assert(raw.Me, Equals, Id(-11203))
	c.Assert(raw.Others, DeepEquals, []Id{190152, 96775, 2344359, 99811, 56867})
}

func (t *T) TestReplaceConfigWithoutMe(c *C) {
	// given
	raw := commCfg.cluster
	// remove me, add one
	newMember := []Id{190152, -2534, 96775, 2344359, 99811}

	// when
	raw.replaceTo(newMember)

	// then
	c.Assert(raw.Me, Equals, Id(-11203))
	c.Assert(raw.Others, DeepEquals, []Id{190152, -2534, 96775, 2344359, 99811})
}