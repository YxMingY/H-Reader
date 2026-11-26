package llmkit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type streamActivityKey struct{}

// WithStreamTimeouts caps the entire request and resets the inactivity timer
// when a streamed response frame arrives.
func WithStreamTimeouts(parent context.Context, first, idle, total time.Duration) (context.Context, context.CancelFunc) {
	totalCtx, stopTotal := context.WithTimeoutCause(parent, total, fmt.Errorf("流式请求达到总时长上限（%s）：%w", total, context.DeadlineExceeded))
	ctx, cancel := context.WithCancelCause(totalCtx)
	var mu sync.Mutex
	deadline := time.Now().Add(first)
	waitingForFirst := true
	var timer *time.Timer
	mu.Lock()
	timer = time.AfterFunc(first, func() {
		mu.Lock()
		defer mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if remaining := time.Until(deadline); remaining > 0 {
			timer.Reset(remaining)
			return
		}
		if waitingForFirst {
			cancel(fmt.Errorf("等待模型首包超时（%s）：%w", first, context.DeadlineExceeded))
		} else {
			cancel(fmt.Errorf("模型连续 %s 未返回数据：%w", idle, context.DeadlineExceeded))
		}
	})
	mu.Unlock()

	activity := func() {
		mu.Lock()
		defer mu.Unlock()
		if ctx.Err() == nil {
			waitingForFirst = false
			deadline = time.Now().Add(idle)
			timer.Reset(idle)
		}
	}
	return context.WithValue(ctx, streamActivityKey{}, activity), func() {
		mu.Lock()
		cancel(nil)
		timer.Stop()
		mu.Unlock()
		stopTotal()
	}
}
