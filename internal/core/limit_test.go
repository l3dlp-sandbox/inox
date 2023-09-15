package core

import (
	"context"
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestExecutionTimeLimitIntegration(t *testing.T) {

	t.Run("context should not be cancelled faster in the presence of child threads", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_TOTAL_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit},
		}, nil)

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread1 = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			lthread2 = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

}

func TestCPUTimeLimitIntegration(t *testing.T) {

	t.Run("context should be cancelled if all CPU time is spent", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)

		_, err = eval(`
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, ctx.GetClosestState(), false)

		if !assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 5*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}
	})

	t.Run("time spent waiting the locking of a shared object's should not count as CPU time", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)
		state := ctx.GetClosestState()
		obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)

		obj.Share(state)

		locked := make(chan struct{})

		go func() {
			otherCtx := NewContexWithEmptyState(ContextConfig{}, nil)
			obj.Lock(otherCtx.state)
			locked <- struct{}{}
			defer close(locked)

			time.Sleep(100 * time.Millisecond)

			obj.Unlock(otherCtx.state)
		}()

		<-locked

		start := time.Now()
		obj.Lock(state)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 2*time.Millisecond) {
			return
		}

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("time spent sleeping should not count as CPU time", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)

		Sleep(ctx, Duration(100*time.Millisecond))

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("time spent waiting to continue after yielding should not count as CPU time", func(t *testing.T) {
		CPU_TIME := 50 * time.Millisecond
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(CPU_TIME))
		if !assert.NoError(t, err) {
			return
		}

		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				LThreadPermission{permkind.Create},
			},
		}))
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "yield 0; return 0",
		}))

		lthreadCtx := NewContext(ContextConfig{
			Limits:        []Limit{cpuLimit},
			ParentContext: state.Ctx,
		})

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: &Module{
				MainChunk:  chunk,
				ModuleKind: UserLThreadModule,
			},
			//prevent the lthread to continue after yielding
			PauseAfterYield: true,
			LthreadCtx:      lthreadCtx,
		})
		assert.NoError(t, err)

		for !lthread.IsPaused() {
			time.Sleep(10 * time.Millisecond)
		}

		time.Sleep(2 * CPU_TIME)

		select {
		case <-lthreadCtx.Done():
			assert.FailNow(t, lthreadCtx.Err().Error())
		case <-state.Ctx.Done():
			assert.FailNow(t, state.Ctx.Err().Error())
		default:
		}

		if !assert.NoError(t, lthread.ResumeAsync()) {
			return
		}

		_, err = lthread.WaitResult(state.Ctx)
		assert.NoError(t, err)
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we do not wait for", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit},
		}, nil)

		state := ctx.GetClosestState()

		res, err := eval(`
			return go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
		`, state, false)

		state.Ctx.PauseCPUTimeDecrementation()

		if !assert.NoError(t, err) {
			return
		}

		lthread, ok := res.(*LThread)

		if !assert.True(t, ok) {
			return
		}

		select {
		case <-lthread.state.Ctx.Done():
		case <-time.After(200 * time.Millisecond):
			assert.FailNow(t, "lthread not done")
		}

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, lthread.state.Ctx.Err(), context.Canceled) {
			return
		}

		assert.ErrorIs(t, state.Ctx.Err(), context.Canceled)
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we wait for", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit},
		}, nil)

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			return lthread.wait_result!()
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context should be cancelled twice as fast if all CPU time is spent equally by parent thread & child thread", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit},
		}, nil)

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context should not be cancelled faster if child thread does nothing", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit},
		}, nil)

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})
}