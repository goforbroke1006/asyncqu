package asyncqu

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_executor_Wait(t *testing.T) {
	t.Run("negative", func(t *testing.T) {
		t.Run("Wait() for non-started executor should not lock", func(t *testing.T) {
			executor := New()

			executor.AddEnd(Start)

			waitFnPassedCh := make(chan error)
			go func() { waitFnPassedCh <- executor.Wait() }()

			select {
			case <-time.After(2 * time.Second):
				t.Error("Wait() keep goroutine too long time")
				t.FailNow()
			case err := <-waitFnPassedCh:
				// test OK
				assert.ErrorIs(t, ErrExecutorWasNotStarted, err)
			}
		})
	})
}

func Test_executor_AsyncRun(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		t.Run("only start and stop", func(t *testing.T) {
			executor := New()

			executor.AddEnd(Start)

			execErr := executor.AsyncRun(context.TODO())
			_ = executor.Wait()

			assert.NoError(t, execErr)
			assert.Len(t, executor.Errs(), 0)
		})

		t.Run("one step", func(t *testing.T) {
			const (
				labelStep1 = "step-1"
			)

			executor := New()

			executor.Append(labelStep1, func(ctx context.Context, step StepName) error {
				return nil
			}, Start)
			executor.AddEnd(labelStep1)

			execErr := executor.AsyncRun(context.TODO())
			_ = executor.Wait()

			assert.NoError(t, execErr)
			assert.Len(t, executor.Errs(), 0)
		})

		t.Run("many steps", func(t *testing.T) {
			const (
				labelStep1  = StepName("step-1")
				labelStep21 = StepName("step-2-1")
				labelStep22 = StepName("step-2-2")
			)
			// start --> step-1 --> step-2-1  --> end
			//                 \--> step-2-2 /

			executor := New()
			spy := &stepVisitSpy{}

			executor.Append(labelStep1, func(ctx context.Context, step StepName) error {
				spy.Append(step)
				t.Logf("step %s is done", step)
				return nil
			}, Start)
			executor.Append(labelStep21, func(ctx context.Context, step StepName) error {
				time.Sleep(250 * time.Millisecond)
				spy.Append(step)
				t.Logf("step %s is done", step)
				return nil
			}, labelStep1)
			executor.Append(labelStep22, func(ctx context.Context, step StepName) error {
				spy.Append(step)
				t.Logf("step %s is done", step)
				return nil
			}, labelStep1)
			executor.AddEnd(labelStep21, labelStep22)

			execErr := executor.AsyncRun(context.TODO())
			waitErr := executor.Wait()

			assert.NoError(t, execErr)
			assert.NoError(t, waitErr)
			assert.Len(t, executor.Errs(), 0)
			assert.Equal(t, labelStep1, spy.At(0))
			assert.Equal(t, labelStep22, spy.At(1))
			assert.Equal(t, labelStep21, spy.At(2))
		})
	})
}

type stepVisitSpy struct {
	narrative   []StepName
	narrativeMx sync.RWMutex
}

func (spy *stepVisitSpy) Append(step StepName) {
	spy.narrativeMx.Lock()
	spy.narrative = append(spy.narrative, step)
	spy.narrativeMx.Unlock()
}

func (spy *stepVisitSpy) Len() int {
	spy.narrativeMx.RLock()
	length := len(spy.narrative)
	spy.narrativeMx.RUnlock()

	return length
}

func (spy *stepVisitSpy) At(index int) StepName {
	spy.narrativeMx.RLock()
	name := spy.narrative[index]
	spy.narrativeMx.RUnlock()

	return name
}
