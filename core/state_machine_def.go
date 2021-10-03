package core

type StateMachine interface {
	Exec(cmd Command) interface{}
}