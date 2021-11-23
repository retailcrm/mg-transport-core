package stacktrace

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/getsentry/raven-go"
	"github.com/stretchr/testify/suite"

	"github.com/retailcrm/mg-transport-core/core/errortools"
)

type ErrCollectorBuilderTest struct {
	builder *ErrCollectorBuilder
	c       *errortools.Collector
	suite.Suite
}

func TestErrCollectorBuilder(t *testing.T) {
	suite.Run(t, new(ErrCollectorBuilderTest))
}

func (t *ErrCollectorBuilderTest) SetupTest() {
	t.c = errortools.NewCollector()
	client, _ := raven.New("fake dsn")
	t.builder = &ErrCollectorBuilder{AbstractStackBuilder{
		client: client,
		err:    t.c,
	}}
}

func (t *ErrCollectorBuilderTest) TestBuild() {
	t.c.Do(
		errors.New("first"),
		errors.New("second"),
		errors.New("third"))

	stack, err := t.builder.Build().GetResult()
	_, file, _, _ := runtime.Caller(0)

	t.Require().NoError(err)
	t.Require().NotZero(stack)
	t.Assert().Len(stack.Frames, 3)

	for _, frame := range stack.Frames {
		t.Assert().Equal(file, frame.Filename)
		t.Assert().Equal(file, frame.AbsolutePath)
		t.Assert().Equal("go", frame.Function)
		t.Assert().Equal(strings.TrimSuffix(filepath.Base(file), ".go"), frame.Module)
		t.Assert().NotZero(frame.Lineno)
	}
}
