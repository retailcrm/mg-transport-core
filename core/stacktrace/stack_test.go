package stacktrace

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FrameTest struct {
	suite.Suite
}

type stackTest struct {
	suite.Suite
}

type StackTraceTest struct {
	suite.Suite
}

func TestFrame(t *testing.T) {
	suite.Run(t, new(FrameTest))
}

func Test_stack(t *testing.T) {
	suite.Run(t, new(stackTest))
}

func TestStackTrace(t *testing.T) {
	suite.Run(t, new(StackTraceTest))
}

func Test_callers(t *testing.T) {
	stack0 := callers(0)
	stack1 := callers(1)
	stack2 := callers(2)

	assert.Len(t, *stack0, len(*stack1)+1)
	assert.Len(t, *stack1, len(*stack2)+1)
	// First function name starts from capital C because it is actually runtime.Callers function - not ours "callers" method.
	// The last one is the test case itself.
	assert.Equal(t, "Callers", string(function((*stack0)[0])))
	assert.Equal(t, "callers", string(function((*stack1)[0])))
	assert.Equal(t, "Test_callers", string(function((*stack2)[0])))
}

func Test_function(t *testing.T) {
	assert.Equal(t, dunno, function(uintptr(9000000000)))
	assert.Equal(t, "Test_function", string(function((*callers(2))[0])))
	assert.Equal(t, "tRunner", string(function((*callers(3))[0])))
}

func Test_source(t *testing.T) {
	assert.Equal(t, dunno, source([][]byte{}, 0))
	assert.Equal(t, dunno, source([][]byte{}, 1))
	assert.Equal(t, []byte("test"), source([][]byte{[]byte("test")}, 1))
}

func Test_funcname(t *testing.T) {
	assert.Equal(t, "c", funcname("a/b.c"))
}

func (t *FrameTest) Test_pc() {
	t.Assert().Equal(uintptr(0), Frame(uintptr(1)).pc())
}

func (t *FrameTest) Test_file() {
	t.Assert().Equal(t.fakeFrame().file(), "unknown")
	t.Assert().Contains(t.frame().file(), "core/stacktrace/stack_test.go")
}

func (t *FrameTest) Test_line() {
	t.Assert().Equal(0, t.fakeFrame().line())
	t.Assert().True(t.frame().line() > 0)
}

func (t *FrameTest) Test_name() {
	t.Assert().Equal("unknown", t.fakeFrame().name())
	t.Assert().Contains(t.frame().name(), "Test_name")
}

func (t *FrameTest) Test_Format() {
	t.Assert().Equal("stack_test.go", fmt.Sprintf("%s", t.frame()))
	t.Assert().Contains(fmt.Sprintf("%+s", t.frame()), "stacktrace.(*FrameTest).Test_Format")
	t.Assert().Contains(fmt.Sprintf("%+s", t.frame()), "core/stacktrace/stack_test.go")
	t.Assert().Equal(strconv.Itoa(t.frame().line()), fmt.Sprintf("%d", t.frame()))
	t.Assert().Equal("(*FrameTest).Test_Format", fmt.Sprintf("%n", t.frame()))
	t.Assert().Equal(fmt.Sprintf("stack_test.go:%d", t.frame().line()), fmt.Sprintf("%v", t.frame()))
}

func (t *FrameTest) Test_MarshalText_Invalid() {
	data, err := t.fakeFrame().MarshalText()

	t.Require().NoError(err)
	t.Assert().Equal("unknown", string(data))
}

func (t *FrameTest) Test_MarshalText() {
	data, err := t.frame().MarshalText()

	t.Require().NoError(err)
	t.Assert().Contains(string(data), "stacktrace.(*FrameTest).Test_MarshalText")
	t.Assert().Contains(string(data), "core/stacktrace/stack_test.go")
}

func (t *FrameTest) frame() Frame {
	return Frame((*callers(3))[0])
}

func (t *FrameTest) fakeFrame() Frame {
	return Frame(9000000001)
}
