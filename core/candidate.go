package core

type Candidate struct{ RaftBase }

func (c *Candidate) TakeAction(msg Msg) Msg {
	return Msg{}
}