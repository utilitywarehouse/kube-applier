package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/client"
)

// strongboxInterface holds functions to configure strongbox for waybill runs
type StrongboxInterface interface {
	SetupGitConfigForStrongbox(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, homeDir string) error
	SetupStrongboxKeyring(ctx context.Context, kubeClient *client.Client, waybill *kubeapplierv1alpha1.Waybill, homeDir string) error
}

type strongboxBase struct{}

func (sb *strongboxBase) SetupStrongboxKeyring(ctx context.Context, kubeClient *client.Client, waybill *kubeapplierv1alpha1.Waybill, homeDir string) error {
	if waybill.Spec.StrongboxKeyringSecretRef == nil {
		return nil
	}
	sbNamespace := waybill.Spec.StrongboxKeyringSecretRef.Namespace
	if sbNamespace == "" {
		sbNamespace = waybill.Namespace
	}
	secret, err := kubeClient.GetSecret(ctx, sbNamespace, waybill.Spec.StrongboxKeyringSecretRef.Name)
	if err != nil {
		return err
	}
	if err := checkSecretIsAllowed(waybill, secret); err != nil {
		return err
	}
	strongboxData, ok := secret.Data[".strongbox_keyring"]
	if !ok {
		return fmt.Errorf(`secret "%s/%s" does not contain key '.strongbox_keyring'`, secret.Namespace, secret.Name)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".strongbox_keyring"), strongboxData, 0400); err != nil {
		return err
	}
	return nil
}

type Strongboxer struct {
	strongboxBase
}

func (s *Strongboxer) SetupGitConfigForStrongbox(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, homeDir string) error {
	if waybill.Spec.StrongboxKeyringSecretRef == nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "strongbox", "-git-config")
	cmd.Dir = homeDir
	// Set PATH so we can find strongbox bin and HOME and STRONGBOX_HOME to
	// point to homeDir
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", homeDir),
		fmt.Sprintf("STRONGBOX_HOME=%s", homeDir),
	}
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running strongbox err:%s %w ", stderr, err)
	}

	return nil
}

// Mock Strongboxer for testing
type mockStrongboxer struct {
	strongboxBase
}

func (m *mockStrongboxer) SetupGitConfigForStrongbox(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, homeDir string) error {
	return nil
}
