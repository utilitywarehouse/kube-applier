package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/client"
)

const cmdWaitDelay = 5 * time.Second

// Known prefixes for strongbox-encrypted content (SIV legacy and age armor).
// A keyring Secret whose values carry either prefix was not decrypted by the
// secret-store operator before kube-applier read it.
var encryptedValuePrefixes = []string{
	"# STRONGBOX ENCRYPTED RESOURCE ;",
	"-----BEGIN AGE ENCRYPTED FILE-----",
}

// strongboxInterface holds functions to configure strongbox for waybill runs
type StrongboxInterface interface {
	SetupGitConfigForStrongbox(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, env []string) error
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
	for k, v := range secret.Data {
		for _, prefix := range encryptedValuePrefixes {
			if strings.HasPrefix(string(v), prefix) {
				return fmt.Errorf("strongbox keyring Secret %s/%s key %q appears to still be encrypted; check that it has been decrypted before use", secret.Namespace, secret.Name, k)
			}
		}
	}
	keyring, ok1 := secret.Data[".strongbox_keyring"]
	if ok1 {
		if err := os.WriteFile(filepath.Join(homeDir, ".strongbox_keyring"), keyring, 0400); err != nil {
			return err
		}
	}
	identity, ok2 := secret.Data[".strongbox_identity"]
	if ok2 {
		if err := os.WriteFile(filepath.Join(homeDir, ".strongbox_identity"), identity, 0400); err != nil {
			return err
		}
	}
	if !ok1 && !ok2 {
		return fmt.Errorf(`secret "%s/%s" does not contain key '.strongbox_keyring' or '.strongbox_identity'`, secret.Namespace, secret.Name)
	}
	return nil
}

type Strongboxer struct {
	strongboxBase
}

func (s *Strongboxer) SetupGitConfigForStrongbox(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, env []string) error {
	if waybill.Spec.StrongboxKeyringSecretRef == nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "strongbox", "-git-config")
	// force kill command 5 seconds after sending it sigterm (when ctx is cancelled/timed out)
	cmd.WaitDelay = cmdWaitDelay
	// Set PATH so we can find strongbox bin
	cmd.Env = append(env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running strongbox err:%s %w ", stderr, err)
	}

	return nil
}
