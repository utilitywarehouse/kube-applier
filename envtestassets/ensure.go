// Package envtestassets ensures envtest binaries (etcd, kube-apiserver) are
// available before tests run. It wraps setup-envtest to manage the asset
// directory, matching the behaviour of the repository Makefile.
package envtestassets

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	once       sync.Once
	assetsPath string
	assetsErr  error
)

// Version is the envtest Kubernetes version used by the test suites.
const Version = "1.30.x"

// Main is a TestMain helper that ensures envtest assets are available
// before running tests. Use it from each package that needs envtest:
//
//	func TestMain(m *testing.M) {
//	    envtestassets.Main(m)
//	}
func Main(m *testing.M) {
	if _, err := EnsureAssets(Version); err != nil {
		fmt.Fprintf(os.Stderr, "envtestassets: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// EnsureAssets ensures that envtest binaries for the given version are
// available. It returns the path to the asset directory, and sets
// KUBEBUILDER_ASSETS so that controller-runtime's envtest finds them.
//
// If KUBEBUILDER_ASSETS is already set and points to a valid directory
// containing etcd and kube-apiserver, it is returned as-is. This makes
// the helper a no-op when the variable has been pre-set (e.g. via 'make
// test').
//
// The result is cached via sync.Once so repeated calls in the same
// process are cheap.
func EnsureAssets(version string) (string, error) {
	once.Do(func() {
		assetsPath, assetsErr = ensureAssets(version)
	})
	return assetsPath, assetsErr
}

func ensureAssets(version string) (string, error) {
	// If already set and valid, use it as-is.
	if p := os.Getenv("KUBEBUILDER_ASSETS"); p != "" {
		if valid, err := dirHasBinaries(p); err == nil && valid {
			return p, nil
		}
	}

	// Determine cache directory (repo's kubebuilder-bindir if we can
	// find the repo root, otherwise $HOME/.cache/kube-applier-envtest).
	cacheDir, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("envtestassets: determine cache dir: %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("envtestassets: mkdir cache dir: %w", err)
	}

	// Acquire a cross-process file lock so that parallel go test
	// processes (run, client, webserver) don't race on setup-envtest.
	lockPath := filepath.Join(cacheDir, ".envtest.lock")
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", fmt.Errorf("envtestassets: open lock file: %w", err)
	}
	if err := lockFile(lf); err != nil {
		lf.Close()
		return "", fmt.Errorf("envtestassets: acquire lock: %w", err)
	}
	defer func() {
		unlockFile(lf)
		lf.Close()
	}()

	// Run setup-envtest to resolve / download the assets.
	// With the lock held only one process does this at a time; when
	// assets are already cached setup-envtest returns immediately.
	path, err := runSetupEnvtest(cacheDir, version)
	if err != nil {
		return "", fmt.Errorf("envtestassets: %w", err)
	}

	// Resolve symlinks (setup-envtest prints the symlink path).
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}

	if valid, err := dirHasBinaries(resolved); err != nil || !valid {
		return "", fmt.Errorf(
			"envtestassets: asset dir %q does not contain expected "+
				"binaries (etcd, kube-apiserver): %v", resolved, err,
		)
	}

	os.Setenv("KUBEBUILDER_ASSETS", resolved)
	return resolved, nil
}

// dirHasBinaries checks that dir is a directory containing etcd and
// kube-apiserver executables.
func dirHasBinaries(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("not a directory")
	}
	for _, name := range []string{"etcd", "kube-apiserver"} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return false, fmt.Errorf("missing %s: %w", name, err)
		}
		if fi.Mode()&0111 == 0 {
			return false, fmt.Errorf("%s is not executable", name)
		}
	}
	return true, nil
}

// cacheDir determines the cache directory for envtest assets. It
// prefers the repo root's kubebuilder-bindir (found by walking up from
// CWD looking for go.mod), falling back to
// $HOME/.cache/kube-applier-envtest.
func cacheDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return defaultCacheDir()
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "kubebuilder-bindir"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return defaultCacheDir()
}

func defaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "kube-applier-envtest"), nil
}

// runSetupEnvtest invokes setup-envtest to resolve the asset path for
// the given version, using the specified cache directory. It prefers
// setup-envtest on PATH, falling back to
// "go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest".
func runSetupEnvtest(cacheDir, version string) (string, error) {
	var args []string

	if p, err := exec.LookPath("setup-envtest"); err == nil {
		args = append([]string{p}, "--bin-dir", cacheDir, "use", "-p", "path", version)
	} else {
		// Use go run -- setup-envtest is not on PATH.
		args = []string{
			"go", "run", "sigs.k8s.io/controller-runtime/tools/setup-envtest@latest",
			"--bin-dir", cacheDir, "use", "-p", "path", version,
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("setup-envtest failed: %s\nstderr: %s",
				err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("setup-envtest failed: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
