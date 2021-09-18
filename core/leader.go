package core

type Leader struct {
	RaftBase
	nextIndex  []Index
	matchIndex []Index
}