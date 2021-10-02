package core

type StateMachine interface {
	exec(cmd Command) interface{}
}