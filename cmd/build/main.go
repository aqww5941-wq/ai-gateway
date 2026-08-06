package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	targetAll      = "all"
	targetFrontend = "frontend"
	targetBackend  = "backend"
	targetClean    = "clean"
)

func main() {
	target := flag.String("target", targetAll, "build target: all, frontend, backend, or clean")
	flag.Parse()

	root, err := findRepoRootFromWorkingDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := runTarget(ctx, root, *target, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func runTarget(ctx context.Context, root, target string, stdout, stderr io.Writer) error {
	switch target {
	case targetAll:
		if err := buildFrontend(ctx, root, stdout, stderr); err != nil {
			return err
		}
		return buildBackend(ctx, root, stdout, stderr)
	case targetFrontend:
		return buildFrontend(ctx, root, stdout, stderr)
	case targetBackend:
		return buildBackend(ctx, root, stdout, stderr)
	case targetClean:
		return cleanLocalArtifacts(root)
	default:
		return fmt.Errorf("unknown target %q (want all, frontend, backend, or clean)", target)
	}
}

func buildFrontend(ctx context.Context, root string, stdout, stderr io.Writer) error {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("find npm: %w", err)
	}
	webDir := filepath.Join(root, "web")
	if err := runCommand(ctx, webDir, nil, stdout, stderr, npm, "ci"); err != nil {
		return fmt.Errorf("install frontend dependencies: %w", err)
	}
	if err := runCommand(ctx, webDir, nil, stdout, stderr, npm, "run", "build"); err != nil {
		return fmt.Errorf("build frontend: %w", err)
	}
	return nil
}

func buildBackend(ctx context.Context, root string, stdout, stderr io.Writer) error {
	goBin, err := findGoTool()
	if err != nil {
		return err
	}
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}
	output := filepath.Join(binDir, gatewayBinaryName(runtime.GOOS))
	env := withEnv(os.Environ(), "CGO_ENABLED", "0")
	args := []string{"build", "-trimpath", "-buildvcs=false", "-o", output, "./cmd/gateway"}
	if err := runCommand(ctx, root, env, stdout, stderr, goBin, args...); err != nil {
		return fmt.Errorf("build gateway: %w", err)
	}
	_, _ = fmt.Fprintln(stdout, "gateway binary:", output)
	return nil
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, key) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, prefix+value)
}

func cleanLocalArtifacts(root string) error {
	// The embedded frontend at internal/static/dist is the canonical, tracked
	// generated asset and is intentionally preserved. Clean removes only ignored
	// local outputs and the retired web/dist location.
	for _, rel := range []string{"bin", filepath.Join("web", "dist")} {
		path := filepath.Join(root, rel)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", rel, err)
		}
	}
	return nil
}

func runCommand(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("%s %s: %w", filepath.Base(name), strings.Join(args, " "), err)
	}
	return nil
}

func findGoTool() (string, error) {
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	path, pathErr := exec.LookPath(name)
	if pathErr == nil {
		return path, nil
	}
	// cmd/build is an ephemeral, same-host build runner. This fallback keeps
	// absolute `go run ./cmd/build` invocations working when Go is not on PATH;
	// the resulting helper binary is never distributed to another machine.
	//lint:ignore SA1019 see the same-host build-runner invariant above
	candidate := filepath.Join(runtime.GOROOT(), "bin", name)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return "", fmt.Errorf("find go tool: %w", pathErr)
}

func gatewayBinaryName(goos string) string {
	if goos == "windows" {
		return "gateway.exe"
	}
	return "gateway"
}

func findRepoRootFromWorkingDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return findRepoRoot(wd)
}

func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start directory: %w", err)
	}
	for {
		if isFile(filepath.Join(dir, "go.mod")) && isFile(filepath.Join(dir, "web", "package.json")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found from %s", start)
		}
		dir = parent
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
