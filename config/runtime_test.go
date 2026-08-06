package config

import (
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
)

func TestValidateReloadBoundaryClassifiesDynamicAndRestartRequiredSections(t *testing.T) {
	current := validTestConfig(t)

	dynamic := Clone(current)
	dynamic.Providers[0].Models = append(dynamic.Providers[0].Models, "model-b")
	dynamic.Routes[0].Targets[0].Model = "model-b"
	if err := ValidateReloadBoundary(current, dynamic); err != nil {
		t.Fatalf("dynamic provider/route change rejected: %v", err)
	}
	if got, want := DynamicReloadSections(current, dynamic), []string{"providers", "routes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DynamicReloadSections() = %v, want %v", got, want)
	}

	restart := Clone(current)
	restart.Server.Port++
	restart.Auth.Enabled = !restart.Auth.Enabled
	restart.RateLimit.Enabled = !restart.RateLimit.Enabled
	restart.Quota.Enabled = !restart.Quota.Enabled
	restart.Cache.MaxSize++
	restart.Tracing.Enabled = !restart.Tracing.Enabled
	restart.Filter.Enabled = !restart.Filter.Enabled
	err := ValidateReloadBoundary(current, restart)
	want := []string{"server", "auth", "rate_limit", "quota", "cache", "tracing", "filter"}
	sections, ok := RestartRequiredSections(err)
	if !ok || !reflect.DeepEqual(sections, want) {
		t.Fatalf("RestartRequiredSections() = %v, %v, want %v, true (error: %v)", sections, ok, want, err)
	}
	var typed *RestartRequiredError
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T, want *RestartRequiredError", err)
	}
}

func TestCloneIsDeepEnoughForImmutableRuntimePublication(t *testing.T) {
	original := validTestConfig(t)
	original.Auth.Keys = []KeyConfig{{Name: "admin", Token: "invalid-example-token", Role: "admin"}}
	original.Filter.Rules = []string{"email"}
	clone := Clone(original)

	clone.Auth.Keys[0].Name = "changed"
	clone.Filter.Rules[0] = "phone_cn"
	clone.Providers[0].Models[0] = "changed-model"
	clone.Routes[0].Targets[0].Model = "changed-model"

	if original.Auth.Keys[0].Name != "admin" || original.Filter.Rules[0] != "email" || original.Providers[0].Models[0] != "model-a" || original.Routes[0].Targets[0].Model != "model-a" {
		t.Fatalf("Clone() shares mutable slices with original: %#v", original)
	}
}

func TestReloaderPublishesConfigOnlyAfterCallbackSuccess(t *testing.T) {
	initial := validTestConfig(t)
	candidate := Clone(initial)
	candidate.Routes[0].Name = "candidate"
	wantErr := errors.New("candidate rejected")
	reloader := &Reloader{
		cfg:      initial,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		onReload: func(*Config) error { return wantErr },
	}

	if err := reloader.applyCandidate(candidate); !errors.Is(err, wantErr) {
		t.Fatalf("applyCandidate() error = %v, want %v", err, wantErr)
	}
	if got := reloader.Config(); !reflect.DeepEqual(got, initial) {
		t.Fatalf("Config() changed after rejected callback: got %#v, want %#v", got, initial)
	}

	reloader.onReload = func(*Config) error { return nil }
	if err := reloader.applyCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	got := reloader.Config()
	if !reflect.DeepEqual(got, candidate) {
		t.Fatalf("Config() = %#v, want accepted candidate %#v", got, candidate)
	}
	got.Routes[0].Name = "external-mutation"
	if reloaded := reloader.Config(); reloaded.Routes[0].Name != "candidate" {
		t.Fatalf("Config() exposed mutable state: %#v", reloaded.Routes[0])
	}
}
