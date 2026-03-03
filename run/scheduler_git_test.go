package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/git"
)

// TestWaybillsWithGitChanges tests the git change detection logic in isolation.
// No envtest, no goroutines, no channels — the Scheduler is constructed directly
// without calling Start(), so there are no background loops that could race with
// the test.
func TestWaybillsWithGitChanges(t *testing.T) {
	ctx := context.Background()
	testRepo, repoPath, hashes := createTestGitRepository(t)

	headHash, err := testRepo.HashForPath(ctx, repoPath)
	require.NoError(t, err)
	require.Equal(t, hashes.head, headHash)

	appAHash, err := testRepo.HashForPath(ctx, filepath.Join(repoPath, "app-a"))
	require.NoError(t, err)
	require.Equal(t, hashes.appA, appAHash)

	appAKHash, err := testRepo.HashForPath(ctx, filepath.Join(repoPath, "app-a-kustomize"))
	require.NoError(t, err)
	require.Equal(t, hashes.appAK, appAKHash)

	staleHash := hashes.initial

	now := metav1.NewTime(time.Now())

	makeScheduler := func(waybills map[string]*kubeapplierv1alpha1.Waybill, lastHash string) *Scheduler {
		s := &Scheduler{
			Repository:        testRepo,
			RepoPath:          repoPath,
			waybills:          waybills,
			gitLastQueuedHash: lastHash,
		}
		return s
	}

	namespaces := func(wbs []*kubeapplierv1alpha1.Waybill) []string {
		ns := make([]string, len(wbs))
		for i, wb := range wbs {
			ns[i] = wb.Namespace
		}
		sort.Strings(ns)
		return ns
	}

	t.Run("returns nil when hash has not changed", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"any": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "any"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{Commit: staleHash},
				},
			},
		}, headHash) // already at current hash
		result := s.waybillsWithGitChanges()
		assert.Nil(t, result)
	})

	t.Run("updates gitLastQueuedHash to current head", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{}, "")
		s.waybillsWithGitChanges()
		assert.Equal(t, headHash, s.gitLastQueuedHash)
	})

	t.Run("skips Waybill with nil LastRun", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"no-last-run": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "no-last-run"},
			},
		}, "")
		result := s.waybillsWithGitChanges()
		assert.Empty(t, result)
	})

	t.Run("skips Waybill already at current commit", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"up-to-date": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "up-to-date"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   headHash,
						Started:  now,
						Finished: now,
					},
				},
			},
		}, "")
		result := s.waybillsWithGitChanges()
		assert.Empty(t, result)
	})

	t.Run("skips Waybill whose path has no changes", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"scheduler-polling-app-a": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "scheduler-polling-app-a"},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					RepositoryPath: "app-a",
				},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   appAHash,
						Started:  now,
						Finished: now,
					},
				},
			},
		}, "")
		result := s.waybillsWithGitChanges()
		assert.Empty(t, result)
	})

	t.Run("returns Waybill whose path has changed", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"scheduler-polling-app-a-kustomize": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "scheduler-polling-app-a-kustomize"},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					RepositoryPath: "app-a-kustomize",
				},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   staleHash,
						Started:  now,
						Finished: now,
					},
				},
			},
		}, "")
		result := s.waybillsWithGitChanges()
		require.Len(t, result, 1)
		assert.Equal(t, "scheduler-polling-app-a-kustomize", result[0].Namespace)
	})

	t.Run("returns only changed Waybills from a mixed set", func(t *testing.T) {
		s := makeScheduler(map[string]*kubeapplierv1alpha1.Waybill{
			"no-last-run": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "no-last-run"},
			},
			"up-to-date": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "up-to-date"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   headHash,
						Started:  now,
						Finished: now,
					},
				},
			},
			"no-path-change": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "no-path-change"},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					RepositoryPath: "app-a",
				},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   appAHash,
						Started:  now,
						Finished: now,
					},
				},
			},
			"changed-a": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "changed-a"},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					RepositoryPath: "app-a-kustomize",
				},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   staleHash,
						Started:  now,
						Finished: now,
					},
				},
			},
			"changed-b": {
				ObjectMeta: metav1.ObjectMeta{Namespace: "changed-b"},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					AutoApply:      ptr.To(false), // autoApply=false: still returned, Enqueue drops it
					RepositoryPath: "app-a-kustomize",
				},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Commit:   staleHash,
						Started:  now,
						Finished: now,
					},
				},
			},
		}, "")
		result := s.waybillsWithGitChanges()
		assert.Equal(t, []string{"changed-a", "changed-b"}, namespaces(result))
	})
}

type testGitRepoHashes struct {
	initial string
	head    string
	appA    string
	appAK   string
}

func createTestGitRepository(t *testing.T) (*git.Repository, string, testGitRepoHashes) {
	t.Helper()

	repoPath := t.TempDir()

	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.name", "kube-applier-tests")
	runGit(t, repoPath, "config", "user.email", "kube-applier-tests@example.invalid")

	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "app-a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "app-a-kustomize"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "app-a", "deployment.yaml"), []byte("a-v1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "app-a-kustomize", "kustomization.yaml"), []byte("k-v1\n"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "initial")

	initial := runGit(t, repoPath, "rev-parse", "--short", "HEAD")

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "app-a-kustomize", "kustomization.yaml"), []byte("k-v2\n"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "update app-a-kustomize")

	head := runGit(t, repoPath, "rev-parse", "--short", "HEAD")

	testRepo, err := git.NewRepository(repoPath, git.RepositoryConfig{Remote: "local"}, git.SyncOptions{})
	require.NoError(t, err)

	ctx := context.Background()
	appAHash, err := testRepo.HashForPath(ctx, filepath.Join(repoPath, "app-a"))
	require.NoError(t, err)
	appAKHash, err := testRepo.HashForPath(ctx, filepath.Join(repoPath, "app-a-kustomize"))
	require.NoError(t, err)

	return testRepo, repoPath, testGitRepoHashes{
		initial: initial,
		head:    head,
		appA:    appAHash,
		appAK:   appAKHash,
	}
}

func runGit(t *testing.T, repoPath string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(output))

	return strings.TrimSpace(string(output))
}
