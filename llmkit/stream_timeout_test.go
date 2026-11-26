package llmkit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamTimeoutsDetectFirstFrameIdleAndTotal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mode  string
		cause string
	}{
		{"first frame", "no-frame", "等待模型首包超时"},
		{"idle", "one-frame", "未返回数据"},
		{"total", "continuous", "总时长上限"},
		{"completes with activity", "finite", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher := w.(http.Flusher)
				write := func(data string) {
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
				if tc.mode != "no-frame" {
					write(`{"choices":[{"delta":{"content":"hello"}}]}`)
				} else {
					flusher.Flush()
				}
				if tc.mode == "no-frame" || tc.mode == "one-frame" {
					select {
					case <-r.Context().Done():
					case <-done:
					}
					return
				}
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				for i := 1; ; i++ {
					select {
					case <-r.Context().Done():
						return
					case <-ticker.C:
						// Frames without visible text still indicate a live stream.
						write(`{"choices":[{"delta":{}}]}`)
						if tc.mode == "finite" && i == 7 {
							write("[DONE]")
							return
						}
					}
				}
			}))
			defer server.Close()
			defer close(done)
			client, err := NewClient(Config{APIKey: "test", BaseURL: server.URL, Model: "test"})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := WithStreamTimeouts(context.Background(), 100*time.Millisecond, 85*time.Millisecond, 240*time.Millisecond)
			defer cancel()
			conv := client.NewConversation()
			conv.AddText("test")
			var reply string
			err = conv.SendStream(ctx, func(chunk string) error { reply += chunk; return nil })
			cause := context.Cause(ctx)
			if tc.cause == "" {
				if err != nil || cause != nil || reply != "hello" || len(conv.History) != 2 {
					t.Fatalf("healthy stream failed: %v, %v, %q", err, cause, reply)
				}
				return
			}
			if err == nil || !errors.Is(cause, context.DeadlineExceeded) || !strings.Contains(cause.Error(), tc.cause) || len(conv.History) != 0 {
				t.Fatalf("unexpected timeout: err=%v cause=%v history=%d", err, cause, len(conv.History))
			}
		})
	}
}
