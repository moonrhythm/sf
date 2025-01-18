package sf

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

var disabled uint32

func SetDisable(value bool) {
	if value {
		atomic.StoreUint32(&disabled, 1)
	} else {
		atomic.StoreUint32(&disabled, 0)
	}
}

func isDisabled() bool {
	return atomic.LoadUint32(&disabled) == 1
}

type groupItem struct {
	cancel context.CancelFunc
	count  uint64
}

var (
	g          singleflight.Group
	mu         sync.Mutex
	shareGroup = make(map[string]*groupItem)
)

func Do[T any](ctx context.Context, key string, fn func(context.Context) (T, error)) (T, error, bool) {
	if isDisabled() {
		r, err := fn(ctx)
		return r, err, false
	}

	var (
		r      any
		err    error
		shared bool
		done   = make(chan struct{})
	)

	mu.Lock()
	if shareGroup[key] == nil {
		shareGroup[key] = &groupItem{}
	}
	sg := shareGroup[key]
	sg.count++
	go func() {
		r, err, shared = g.Do(key, func() (any, error) {
			var nctx context.Context
			nctx, sg.cancel = context.WithCancel(context.WithoutCancel(ctx))
			return fn(nctx)
		})
		close(done)
	}()
	mu.Unlock()

	select {
	case <-ctx.Done():
	case <-done:
	}

	mu.Lock()
	sg.count--
	if sg.count == 0 {
		delete(shareGroup, key)
		if sg.cancel != nil {
			sg.cancel()
		}
	}
	mu.Unlock()

	if err != nil {
		return *new(T), err, false
	}
	return r.(T), nil, shared
}

func DoVoid(ctx context.Context, key string, fn func(context.Context) error) (error, bool) {
	_, err, shared := Do(ctx, key, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err, shared
}

func Forget(key string) {
	mu.Lock()
	delete(shareGroup, key)
	g.Forget(key)
	mu.Unlock()
}
