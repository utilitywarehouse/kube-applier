package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
)

// TestUpdateRepoBaseAddresses covers updateRepoBaseAddresses, which walks
// tmpRepoDir for kustomization files and rewrites ssh://github.com URLs
// to ssh://<keyname>_github_com/... when preceded by a
// # kube-applier: key_<name> marker comment.
func TestUpdateRepoBaseAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "key_deploy marker rewrites ssh url",
			input: `bases:
  # kube-applier: key_deploy
  - ssh://github.com/utilitywarehouse/kube-applier//testdata/bases/simple-deployment?ref=master
resources:
  - 00-namespace.yaml
`,
			expected: `bases:
  # kube-applier: key_deploy
  - ssh://deploy_github_com/utilitywarehouse/kube-applier//testdata/bases/simple-deployment?ref=master
resources:
  - 00-namespace.yaml
`,
		},
		{
			name: "marker applies only to immediately following ssh:// line",
			input: `bases:
  # kube-applier: key_other
  - ssh://github.com/other/repo//path?ref=main
  - ssh://github.com/unmodified/repo//path
resources:
  - 00-namespace.yaml
`,
			expected: `bases:
  # kube-applier: key_other
  - ssh://other_github_com/other/repo//path?ref=main
  - ssh://github.com/unmodified/repo//path
resources:
  - 00-namespace.yaml
`,
		},
		{
			name: "github.com without ssh:// prefix is not rewritten",
			input: `bases:
  # kube-applier: key_deploy
  - github.com/foo/bar
resources:
  - 00-namespace.yaml
`,
			expected: `bases:
  # kube-applier: key_deploy
  - github.com/foo/bar
resources:
  - 00-namespace.yaml
`,
		},
		{
			name: "marker followed by non-url line is consumed without error",
			input: `bases:
  # kube-applier: key_deploy
  - some-non-url-content
resources:
  - 00-namespace.yaml
`,
			expected: `bases:
  # kube-applier: key_deploy
  - some-non-url-content
resources:
  - 00-namespace.yaml
`,
		},
		{
			name: "resources: entries with markers are also rewritten",
			input: `resources:
  # kube-applier: key_deploy
  - ssh://github.com/org/repo//manifests?ref=main
`,
			expected: `resources:
  # kube-applier: key_deploy
  - ssh://deploy_github_com/org/repo//manifests?ref=main
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			err := os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"),
				[]byte(tt.input), 0644)
			assert.NoError(t, err)

			err = (&Runner{}).updateRepoBaseAddresses(tmpDir)
			assert.NoError(t, err)

			got, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}

// TestConstructSSHConfig covers constructSSHConfig, which builds an SSH config
// body and writes key_<name> files into sshDir.
func TestConstructSSHConfig(t *testing.T) {
	t.Parallel()

	t.Run("single key adds Host github.com fallback", func(t *testing.T) {
		sshDir := t.TempDir()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-ssh",
				Namespace: "test-ns",
			},
			Data: map[string][]byte{
				"key_deploy": []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----"),
			},
		}
		configFile := filepath.Join(sshDir, "config")
		keyFile := filepath.Join(sshDir, "key_deploy")
		body, err := (&Runner{}).constructSSHConfig(secret, sshDir, configFile)
		assert.NoError(t, err)

		// Verify key file was written with trailing newline
		keyData, err := os.ReadFile(keyFile)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(string(keyData), "\n"),
			"key file should end with newline")
		assert.Contains(t, string(keyData),
			"-----BEGIN OPENSSH PRIVATE KEY-----")

		// Verify config body contains both host blocks
		expectedDeployBlock := "Host deploy_github_com\n    HostName github.com\n    IdentitiesOnly yes\n    IdentityFile " + keyFile + "\n    User git\n"
		expectedFallbackBlock := "Host github.com\n    HostName github.com\n    IdentitiesOnly yes\n    IdentityFile " + keyFile + "\n    User git\n"
		expectedBody := expectedDeployBlock + "\n" + expectedFallbackBlock
		assert.Equal(t, expectedBody, string(body))
	})

	t.Run("two keys produce two host blocks without fallback", func(t *testing.T) {
		sshDir := t.TempDir()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-ssh",
				Namespace: "test-ns",
			},
			Data: map[string][]byte{
				"key_deploy": []byte("key1-content"),
				"key_other":  []byte("key2-content"),
			},
		}
		configFile := filepath.Join(sshDir, "config")
		body, err := (&Runner{}).constructSSHConfig(secret, sshDir, configFile)
		assert.NoError(t, err)

		// Both key files should exist
		assert.FileExists(t, filepath.Join(sshDir, "key_deploy"))
		assert.FileExists(t, filepath.Join(sshDir, "key_other"))

		// Check that both host blocks appear in the body regardless of
		// map iteration order.
		bodyStr := string(body)
		deployBlock := "Host deploy_github_com\n    HostName github.com\n    IdentitiesOnly yes\n    IdentityFile " +
			filepath.Join(sshDir, "key_deploy") + "\n    User git\n"
		otherBlock := "Host other_github_com\n    HostName github.com\n    IdentitiesOnly yes\n    IdentityFile " +
			filepath.Join(sshDir, "key_other") + "\n    User git\n"
		assert.Contains(t, bodyStr, deployBlock)
		assert.Contains(t, bodyStr, otherBlock)
		// No fallback Host github.com block for two keys
		assert.NotContains(t, bodyStr, "\nHost github.com")
	})

	t.Run("zero key entries returns error", func(t *testing.T) {
		sshDir := t.TempDir()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-ssh",
				Namespace: "test-ns",
			},
			Data: map[string][]byte{
				"known_hosts": []byte("some-host-key"),
			},
		}
		_, err := (&Runner{}).constructSSHConfig(secret, sshDir, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(),
			`secret "test-ns/git-ssh" does not contain any keys`)
	})

	t.Run("key without trailing newline gets one appended", func(t *testing.T) {
		sshDir := t.TempDir()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-ssh",
				Namespace: "test-ns",
			},
			Data: map[string][]byte{
				"key_deploy": []byte("no-newline-at-end"),
			},
		}
		configFile := filepath.Join(sshDir, "config")
		_, err := (&Runner{}).constructSSHConfig(secret, sshDir, configFile)
		assert.NoError(t, err)

		keyData, err := os.ReadFile(filepath.Join(sshDir, "key_deploy"))
		assert.NoError(t, err)
		assert.Equal(t, "no-newline-at-end\n", string(keyData))
	})
}

// TestSetupGitSSH_NoSecretRefFallback covers the no-GitSSHSecretRef branches
// of setupGitSSH, which set GIT_SSH_COMMAND using the default key path or
// /dev/null.
func TestSetupGitSSH_NoSecretRefFallback(t *testing.T) {
	t.Parallel()

	t.Run("with default key path", func(t *testing.T) {
		tmpHome := t.TempDir()
		runner := &Runner{
			DefaultGitSSHKeyPath: "/some/key",
		}
		waybill := &kubeapplierv1alpha1.Waybill{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
		}

		env, err := runner.setupGitSSH(context.Background(), waybill, tmpHome)
		assert.NoError(t, err)
		assert.Equal(t,
			"GIT_SSH_COMMAND=ssh -q -F none -o IdentitiesOnly=yes -o User=git -o IdentityFile=/some/key -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no",
			env)
	})

	t.Run("without default key path uses /dev/null", func(t *testing.T) {
		tmpHome := t.TempDir()
		runner := &Runner{}
		waybill := &kubeapplierv1alpha1.Waybill{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
		}

		env, err := runner.setupGitSSH(context.Background(), waybill, tmpHome)
		assert.NoError(t, err)
		assert.Equal(t,
			"GIT_SSH_COMMAND=ssh -q -F none -o IdentitiesOnly=yes -o IdentityFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no",
			env)
	})
}
