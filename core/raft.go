package core

type Id int64
type Term int64
type Index int64
type Command string

type Entry struct {
	term Term
	cmd Command
}

type RaftObject struct {
	currentTerm Id
	votedFor Term
	commitIndex Index
	lastApplied Index
	log []Entry
}

type Follower RaftObject
type Candidate RaftObject
type Leader struct {
	base RaftObject
	nextIndex []Index
	matchIndex []Index
}

