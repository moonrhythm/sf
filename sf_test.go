package sf_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moonrhythm/sf"
)

func TestDo(t *testing.T) {
	var cnt uint64

	f := func() int {
		atomic.AddUint64(&cnt, 1)
		time.Sleep(10 * time.Millisecond)
		return 1
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			sf.Do(context.Background(), "key", func(ctx context.Context) (int, error) { return f(), nil })
			wg.Done()
		}()
	}

	wg.Wait()
	if got, want := atomic.LoadUint64(&cnt), uint64(1); got != want {
		t.Errorf("want 1 got %d", cnt)
	}
}

func TestDo_Canceled(t *testing.T) {
	f := func() int {
		time.Sleep(30 * time.Millisecond)
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	var err error
	done := make(chan struct{})
	go func() {
		_, err, _ = sf.Do(ctx, "key", func(ctx context.Context) (int, error) { return f(), nil })
		close(done)
	}()
	cancel()

	<-done

	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context canceled error")
	}
}

func TestDoVoid(t *testing.T) {
	err, _ := sf.DoVoid(context.Background(), "key", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("want nil got %v", err)
	}
}
