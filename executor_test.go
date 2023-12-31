package asyncqu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_executorImpl_SetOnChanges(t *testing.T) {
	t.Parallel()

	t.Run("negative", func(t *testing.T) {
		t.Run("run without Wait(), catch error", func(t *testing.T) {
			var fakeError = errors.New("fake error")
			const (
				stage1  = StageName("stage-1")
				stage2  = StageName("stage-2")
				stage31 = StageName("stage-3-1")
				stage32 = StageName("stage-3-2")
				stage33 = StageName("stage-3-3")
				stage4  = StageName("stage-4")
			)
			//                              /--> stage-3-1 \
			// start --> stage-1 --> stage-2 --> stage-3-2  --> stage-4 --> end
			//                              \--> stage-3-3 /

			executor := New()

			fnNormal := func(ctx context.Context) error { return nil }
			fnWithErr := func(ctx context.Context) error { return fakeError }

			statesCounter := map[State]int{
				Runnable: 0,
				Running:  0,
				Done:     0,
				Skipped:  0,
			}

			executor.SetOnChanges(func(stageName StageName, state State, err error) {
				if stageName == Start || stageName == End {
					return
				}
				t.Logf("%s %s\n", stageName, state)
				statesCounter[state]++
			})
			executor.Append(stage1, fnNormal, Start)
			executor.Append(stage2, fnWithErr, stage1)
			executor.Append(stage31, fnNormal, stage2)
			executor.Append(stage32, fnNormal, stage2)
			executor.Append(stage33, fnNormal, stage2)
			executor.Append(stage4, fnNormal, stage31, stage32, stage33)
			executor.SetEnd(stage4)

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Equal(t, 6, statesCounter[Runnable]) // six functions registered
			assert.Equal(t, 2, statesCounter[Running])  // stage-1 and stage-2, another are skipped
			assert.Equal(t, 2, statesCounter[Done])     // stage-1 and stage-2, another are skipped
			assert.Equal(t, 4, statesCounter[Skipped])  // stage-31,32,33 and stage-4
		})
	})
}

func Test_executorImpl_Append(t *testing.T) {
	t.Parallel()

	t.Run("negative", func(t *testing.T) {
		t.Run("panic: duplicates", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("The code did not panic")
				}
			}()

			executor := New()
			executor.Append("stage-1", nil, Start)
			executor.Append("stage-1", nil, Start)
		})

		t.Run("panic: stage should not wait for itself", func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("The code did not panic")
					t.FailNow()
				}
				assert.ErrorIs(t, ErrStageShouldNotWaitForItself, r.(error))
			}()

			executor := New()
			executor.Append("stage-1", nil, "stage-1")
		})

		t.Run("panic: stage wait for unknown", func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("The code did not panic")
					t.FailNow()
				}
				assert.ErrorIs(t, ErrStageWaitForUnknown, r.(error))
			}()

			executor := New()
			executor.Append("stage-2", nil, "stage-1")
		})
	})
}

func Test_executor_Run(t *testing.T) {
	t.Parallel()

	t.Run("check END stage", func(t *testing.T) {
		t.Parallel()

		t.Run("negative - no END stage", func(t *testing.T) {
			execCtx, execCancel := context.WithTimeout(context.TODO(), 250*time.Millisecond)
			defer execCancel()

			executor := New()
			executor.Append("some-stage", nil, Start)
			execErr := executor.Run(execCtx)
			assert.ErrorIs(t, execErr, ErrEndStageIsNotSpecified)
		})

		t.Run("positive - no END stage so START == END", func(t *testing.T) {
			execCtx, execCancel := context.WithTimeout(context.TODO(), 250*time.Millisecond)
			defer execCancel()

			executor := New()
			execErr := executor.Run(execCtx)
			assert.NoError(t, execErr)
		})

		t.Run("positive - START == END stage", func(t *testing.T) {
			execCtx, execCancel := context.WithTimeout(context.TODO(), 250*time.Millisecond)
			defer execCancel()

			executor := New()
			executor.SetEnd(Start)

			execErr := executor.Run(execCtx)
			assert.NoError(t, execErr)
		})
	})

	t.Run("negative", func(t *testing.T) {
		t.Run("context canceled before any stage starts", func(t *testing.T) {
			const (
				stage1  = StageName("stage-1-load-users-list")
				stage21 = StageName("stage-2-1-enrich-user-data")
				stage22 = StageName("stage-2-2-load-payments-info")
			)

			spy := NewStageVisitSpy()

			executor := New()
			executor.Append(stage1, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				select {
				case <-ctx.Done():
					// ok
				case <-time.After(1 * time.Second):
					spy.Append(stage)
					t.Logf("stage %s is done", stage)
				}

				return nil
			}, Start)
			executor.Append(stage21, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(250 * time.Millisecond)
				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.Append(stage22, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.SetEnd(stage21, stage22)
			executor.SetFinal(func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)
				spy.Append(stage)
				return nil
			})

			runCtx, runCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer runCancel()

			execErr := executor.Run(runCtx)
			assert.NoError(t, execErr)

			assert.Equal(t, 1, spy.Len())
			assert.Equal(t, Final, spy.At(0))
		})

		t.Run("context canceled after half of stages done", func(t *testing.T) {
			// TIMEOUT = 600 millis
			// stage1 200 millis ---> stage2-1 200 millis --> | TIMEOUT 600 millis |-----------> stage3
			//                   \--> stage2-2 1000 millis----------------------------------/
			// stage2-2 NEVER STARTS

			const (
				stage1  = StageName("stage-1-load-users-list")
				stage21 = StageName("stage-2-1-enrich-user-data")
				stage22 = StageName("stage-2-2-load-payments-info")
				stage3  = StageName("stage-2-2-generate-CSV")
			)

			spy := NewStageVisitSpy()

			executor := New()
			executor.SetFinal(func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)
				spy.Append(stage)
				return nil
			})
			executor.Append(stage1, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(200 * time.Millisecond)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, Start)
			executor.Append(stage21, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(200 * time.Millisecond)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.Append(stage22, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(1000 * time.Millisecond)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.Append(stage3, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(1000 * time.Millisecond)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage21, stage22)
			executor.SetEnd(stage3)

			execCtx, execCancel := context.WithTimeout(context.TODO(), 600*time.Millisecond)
			defer execCancel()

			execErr := executor.Run(execCtx)
			assert.NoError(t, execErr)

			assert.Equal(t, 4, spy.Len())
			assert.Equal(t, stage1, spy.At(0))
			assert.Equal(t, stage21, spy.At(1))
			assert.Equal(t, stage22, spy.At(2))
			assert.Equal(t, Final, spy.At(3))
		})

		t.Run("stage 1 failed", func(t *testing.T) {
			var fakeErr = errors.New("fake error")

			const (
				stage1  = StageName("stage-1")
				stage21 = StageName("stage-2-1")
				stage22 = StageName("stage-2-2")
			)
			// start --> stage-1 --> stage-2-1  --> end
			//                  \--> stage-2-2 /

			executor := New()
			spy := NewStageVisitSpy()

			executor.Append(stage1, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return fakeErr
			}, Start)
			executor.Append(stage21, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				time.Sleep(250 * time.Millisecond)
				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.Append(stage22, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}, stage1)
			executor.SetEnd(stage21, stage22)

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 1)

			assert.Equal(t, 1, spy.Len())
			assert.Equal(t, stage1, spy.At(0))
		})
	})

	t.Run("positive", func(t *testing.T) {
		t.Run("only start and stop", func(t *testing.T) {
			executor := New()

			executor.SetEnd(Start)

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 0)
		})

		t.Run("one stage", func(t *testing.T) {
			const (
				stage1 = "stage-1"
			)

			executor := New()

			executor.Append(stage1, func(ctx context.Context) error {
				return nil
			}, Start)
			executor.SetEnd(stage1)

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 0)
		})

		t.Run("many stages", func(t *testing.T) {
			const (
				stage1  = StageName("stage-1")
				stage21 = StageName("stage-2-1")
				stage22 = StageName("stage-2-2")
			)
			// start --> stage-1 --> stage-2-1  --> end
			//                  \--> stage-2-2 /

			executor := New()
			spy := NewStageVisitSpy()

			fnWithSleep := func(sleep time.Duration) StageFn {
				return func(ctx context.Context) error {
					time.Sleep(sleep)

					stage := ctx.Value(ContextKeyStageName).(StageName)

					spy.Append(stage)
					t.Logf("stage %s is done", stage)
					return nil
				}
			}
			executor.Append(stage1, fnWithSleep(0), Start)
			executor.Append(stage21, fnWithSleep(time.Second), stage1)
			executor.Append(stage22, fnWithSleep(0), stage1)
			executor.SetEnd(stage21, stage22)

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 0)

			assert.Equal(t, 3, spy.Len())
			assert.Equal(t, stage1, spy.At(0))
			assert.Equal(t, stage22, spy.At(1))
			assert.Equal(t, stage21, spy.At(2))
		})

		t.Run("one stage fail and one stuck", func(t *testing.T) {
			const (
				stage1  = StageName("stage-1")
				stage21 = StageName("stage-2-1")
				stage22 = StageName("stage-2-2")
			)
			var fakeErr = errors.New("fake errors")

			spy := NewStageVisitSpy()

			executor := New()
			executor.Append(stage1, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				select {
				case <-ctx.Done():
					// ok
				case <-time.After(100 * time.Millisecond):
					spy.Append(stage)
					t.Logf("stage %s is done", stage)
				}

				return nil
			}, Start)
			executor.Append(stage21, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				select {
				case <-ctx.Done():
					// ok
				case <-time.After(1000 * time.Millisecond):
					spy.Append(stage)
					t.Logf("stage %s is done", stage)
				}

				return fakeErr
			}, stage1)
			executor.Append(stage22, func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				select {
				case <-ctx.Done():
					// ok
				case <-time.After(2000 * time.Millisecond):
					spy.Append(stage)
					t.Logf("stage %s is done", stage)
				}

				return nil
			}, stage1)
			executor.SetEnd(stage21, stage22)

			runCtx, runCancel := context.WithTimeout(context.TODO(), 2500*time.Millisecond)
			defer runCancel()

			runErr := executor.Run(runCtx)
			assert.NoError(t, runErr)

			assert.Equal(t, 3, spy.Len())
			assert.Equal(t, stage1, spy.At(0))
			assert.Equal(t, stage21, spy.At(1))
			assert.Equal(t, stage22, spy.At(2))
		})
	})
}

func Test_executor_SetFinal(t *testing.T) {
	t.Parallel()

	t.Run("positive", func(t *testing.T) {
		t.Run("final cb called even some errors", func(t *testing.T) {
			var fakeErr = errors.New("fake error")

			const (
				stage1  = StageName("stage-1")
				stage2  = StageName("stage-2")
				stage31 = StageName("stage-3-1")
				stage32 = StageName("stage-3-2")
			)
			// start --> stage-1 -- ERROR --> stage-2 --> stage-3-1  --> end
			//                                       \--> stage-3-2 /

			executor := New()
			spy := NewStageVisitSpy()

			fnWithErr := func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return fakeErr
			}

			executor.SetOnChanges(func(stageName StageName, state State, err error) {
				if stageName == Start || stageName == End {
					return
				}
				t.Logf("%s %s\n", stageName, state)
			})
			executor.Append(stage1, fnWithErr, Start)
			executor.Append(stage2, fnWithErr, stage1)
			executor.Append(stage31, fnWithErr, stage2)
			executor.Append(stage32, fnWithErr, stage2)
			executor.SetEnd(stage31, stage32)
			executor.SetFinal(func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Log("final")

				return nil
			})

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 1)

			assert.Equal(t, 2, spy.Len()) // only stage-1 and final
			assert.Equal(t, stage1, spy.At(0))
			assert.Equal(t, Final, spy.At(1))
		})

		t.Run("final cb called after stage without any error", func(t *testing.T) {
			const (
				stage1  = StageName("stage-1")
				stage2  = StageName("stage-2")
				stage31 = StageName("stage-3-1")
				stage32 = StageName("stage-3-2")
			)
			// start --> stage-1 -- ERROR --> stage-2 --> stage-3-1  --> end
			//                                       \--> stage-3-2 /

			executor := New()
			spy := NewStageVisitSpy()

			fnNoErr := func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Logf("stage %s is done", stage)
				return nil
			}

			executor.SetOnChanges(func(stageName StageName, state State, err error) {
				if stageName == Start || stageName == End {
					return
				}
				t.Logf("%s %s\n", stageName, state)
			})
			executor.Append(stage1, fnNoErr, Start)
			executor.Append(stage2, fnNoErr, stage1)
			executor.Append(stage31, fnNoErr, stage2)
			executor.Append(stage32, fnNoErr, stage2)
			executor.SetEnd(stage31, stage32)
			executor.SetFinal(func(ctx context.Context) error {
				stage := ctx.Value(ContextKeyStageName).(StageName)

				spy.Append(stage)
				t.Log("final")

				return nil
			})

			execErr := executor.Run(context.TODO())
			assert.NoError(t, execErr)

			assert.Len(t, executor.Errs(), 0)

			assert.Equal(t, 5, spy.Len()) // only stage-1 and final
			assert.Equal(t, stage1, spy.At(0))
			assert.Equal(t, stage2, spy.At(1))
			assert.True(t, stage31 == spy.At(2) || stage32 == spy.At(2)) // 31 or 32 because it's async
			assert.True(t, stage31 == spy.At(3) || stage32 == spy.At(3)) // 31 or 32 because it's async
			assert.Equal(t, Final, spy.At(4))
		})
	})
}
