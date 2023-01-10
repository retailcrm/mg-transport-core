package healthcheck

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SyncMapStorageTest struct {
	suite.Suite
	storage Storage
}

func TestSyncMapStorage(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SyncMapStorageTest))
}

func (t *SyncMapStorageTest) SetupSuite() {
	t.storage = NewSyncMapStorage(NewAtomicCounter)
}

func (t *SyncMapStorageTest) Test_Get() {
	counter := t.storage.Get(1, "Name")
	t.Assert().NotNil(counter)
	t.Assert().IsType(&AtomicCounter{}, counter)
	t.Assert().Equal("Name", counter.Name())

	newCounter := t.storage.Get(1, "New Name")
	t.Assert().Equal(counter, newCounter)
	t.Assert().Equal("New Name", newCounter.Name())
}

func (t *SyncMapStorageTest) Test_Process() {
	var wg sync.WaitGroup
	wg.Add(1)
	t.storage.Process(storageCallbackProcessor{callback: func(id int, counter Counter) bool {
		t.Assert().Equal(1, id)
		t.Assert().Equal("New Name", counter.Name())
		wg.Done()
		return false
	}})

	wg.Wait()
}

func (t *SyncMapStorageTest) Test_Remove() {
	defer func() {
		if r := recover(); r != nil {
			t.Fail("unexpected panic:", r)
		}
	}()
	t.storage.Remove(0)
	t.storage.Remove(-1)
	t.storage.Remove(1)
	t.storage.Process(storageCallbackProcessor{callback: func(id int, counter Counter) bool {
		t.Fail("did not expect any items:", id, counter)
		return false
	}})
}

type storageCallbackProcessor struct {
	callback func(id int, counter Counter) bool
}

func (p storageCallbackProcessor) Process(id int, counter Counter) bool {
	return p.callback(id, counter)
}
