package errortools

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ErrCollectorTest struct {
	c *Collector
	suite.Suite
}

func TestErrCollector(t *testing.T) {
	suite.Run(t, new(ErrCollectorTest))
}

func TestCollect_NoError(t *testing.T) {
	require.NoError(t, Collect())
}

func TestCollect_NoError_Nils(t *testing.T) {
	require.NoError(t, Collect(nil, nil, nil))
}

func TestCollect_Error(t *testing.T) {
	require.Error(t, Collect(errors.New("first"), errors.New("second")))
}

func (t *ErrCollectorTest) SetupTest() {
	t.c = NewCollector()
}

func (t *ErrCollectorTest) TestDo() {
	t.c.Do(
		errors.New("first"),
		errors.New("second"),
		errors.New("third"))

	t.Assert().False(t.c.OK())
	t.Assert().NotEmpty(t.c.String())
	t.Assert().Error(t.c.AsError())
	t.Assert().Equal(3, t.c.Len())
	t.Assert().Panics(func() {
		t.c.Panic()
	})

	i := 0
	for err := range t.c.Iterate() {
		t.Assert().Error(err.Err)
		t.Assert().NotEmpty(err.File)
		t.Assert().NotZero(err.Line)
		t.Assert().NotZero(err.PC)

		switch i {
		case 0:
			t.Assert().Equal("first", err.Error())
		case 1:
			t.Assert().Equal("second", err.Error())
		case 2:
			t.Assert().Equal("third", err.Error())
		}

		i++
	}
}

func (t *ErrCollectorTest) Test_PanicNone() {
	t.Assert().NotPanics(func() {
		t.c.Panic()
	})
}
