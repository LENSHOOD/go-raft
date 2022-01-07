package comm

import (
	. "gopkg.in/check.v1"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

type fakeWriter struct {
	bBuf []byte
}

func (f *fakeWriter) Write(p []byte) (n int, err error) {
	f.bBuf = p
	return len(p), nil
}

func (t *T) Test_initLogger(c *C) {
	// given
	level := "info"
	writer := &fakeWriter{}

	initLogger(func(l *loggerOption) { l.loggerFile = writer }, WithLoggerLevel(level))

	// when
	writer.bBuf = make([]byte, 0)
	sugarLogger.Infow("info")
	_ = sugarLogger.Sync()

	// then
	c.Assert(string(writer.bBuf), Not(Equals), "")

	// when
	writer.bBuf = make([]byte, 0)
	sugarLogger.Debugw("debug")
	_ = sugarLogger.Sync()

	// then
	c.Assert(string(writer.bBuf), Equals, "")
}
