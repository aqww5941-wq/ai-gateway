package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-gateway/internal/store"
)

func TestQuotaCheckCurrentBaselineAllowsConcurrentBudgetOvershoot(t *testing.T) {
	st := openQuotaTestStore(t)
	token, err := st.CreateKey("concurrent", "user", "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := st.LookupIdentity(token)
	if err != nil || identity == nil {
		t.Fatalf("LookupIdentity() = %#v, %v", identity, err)
	}

	const concurrentRequests = 16
	entered := make(chan struct{}, concurrentRequests)
	release := make(chan struct{})
	handler := QuotaCheck(st, discardLogger(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	statuses := make(chan int, concurrentRequests)
	var requests sync.WaitGroup
	requests.Add(concurrentRequests)
	for requestIndex := 0; requestIndex < concurrentRequests; requestIndex++ {
		go func() {
			defer requests.Done()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, requestWithIdentity(identity))
			statuses <- recorder.Code
		}()
	}

	for requestIndex := 0; requestIndex < concurrentRequests; requestIndex++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("not all requests passed the non-atomic quota check")
		}
	}
	close(release)
	requests.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("concurrent quota status = %d, want current fail-open gate status 200", status)
		}
	}

	// The server records usage only after upstream completion. Applying those
	// completions sequentially makes the overshoot deterministic without
	// conflating it with SQLite writer-lock behavior.
	for requestIndex := 0; requestIndex < concurrentRequests; requestIndex++ {
		if err := st.RecordUsage(identity.ID, 1); err != nil {
			t.Fatal(err)
		}
	}
	allowed, used, limit, err := st.CheckQuota(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed || used != concurrentRequests || limit != 1 {
		t.Fatalf("post-completion quota = allowed %v, used %d, limit %d", allowed, used, limit)
	}
	t.Log("known gap: check-then-record permits concurrent overshoot; Task 51 owns atomic reservation")
}

func TestQuotaCheckCurrentBaselineFailsOpenOnStoreError(t *testing.T) {
	st := openQuotaTestStore(t)
	token, err := st.CreateKey("store-failure", "user", "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := st.LookupIdentity(token)
	if err != nil || identity == nil {
		t.Fatalf("LookupIdentity() = %#v, %v", identity, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	called := false
	handler := QuotaCheck(st, logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, requestWithIdentity(identity))

	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("store failure result = called %v, status %d; want current fail-open behavior", called, recorder.Code)
	}
	logOutput := logs.String()
	if !strings.Contains(logOutput, "quota check failed") {
		t.Fatal("quota failure log is missing the diagnostic message")
	}
	if strings.Contains(logOutput, token) {
		t.Fatal("quota failure log contains the bearer token")
	}
	t.Log("known gap: quota Store failures fail open; Task 45/51 own fail-closed policy and reservation")
}

func openQuotaTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close Store: %v", err)
		}
	})
	return st
}

func requestWithIdentity(identity *store.KeyIdentity) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := context.WithValue(request.Context(), ctxKeyIdentity, identity)
	return request.WithContext(ctx)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}
