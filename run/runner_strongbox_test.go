package run

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
)

func TestVerifyKeyringNotEncrypted(t *testing.T) {
	makeSecret := func(data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "strongbox-keyring", Namespace: "example-ns"},
			Data:       data,
		}
	}

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name:    "plaintext keyring passes",
			secret:  makeSecret(map[string][]byte{".strongbox_keyring": []byte("keyid: abc123\n")}),
			wantErr: false,
		},
		{
			name:    "strongbox SIV-encrypted keyring is rejected",
			secret:  makeSecret(map[string][]byte{".strongbox_keyring": []byte("# STRONGBOX ENCRYPTED RESOURCE ; some-ciphertext")}),
			wantErr: true,
		},
		{
			name:    "age-armored keyring is rejected",
			secret:  makeSecret(map[string][]byte{".strongbox_identity": []byte("-----BEGIN AGE ENCRYPTED FILE-----\nsome-ciphertext\n-----END AGE ENCRYPTED FILE-----")}),
			wantErr: true,
		},
		{
			name:    "empty secret passes",
			secret:  makeSecret(nil),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Replicate the loop added to SetupStrongboxKeyring.
			var err error
			for k, v := range tc.secret.Data {
				for _, prefix := range encryptedValuePrefixes {
					if strings.HasPrefix(string(v), prefix) {
						err = fmt.Errorf("strongbox keyring Secret %s/%s key %q appears to still be encrypted", tc.secret.Namespace, tc.secret.Name, k)
					}
				}
			}
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckSecretIsAllowed(t *testing.T) {
	tests := []struct {
		name        string
		waybillNS   string
		secretNS    string
		annotations map[string]string
		wantErr     string
	}{
		{
			name:      "allows same namespace",
			waybillNS: "app-d",
			secretNS:  "app-d",
		},
		{
			name:      "allows explicit namespace match",
			waybillNS: "app-d-strongbox-shared",
			secretNS:  "app-d",
			annotations: map[string]string{
				secretAllowedNamespacesAnnotation: "app-d-strongbox-shared,other",
			},
		},
		{
			name:      "allows wildcard namespace match",
			waybillNS: "app-d-strongbox-shared-is-allowed",
			secretNS:  "app-d",
			annotations: map[string]string{
				secretAllowedNamespacesAnnotation: "app-d-strongbox-shared-is-*",
			},
		},
		{
			name:      "rejects namespace not in annotation list",
			waybillNS: "app-d-strongbox-shared-not-allowed",
			secretNS:  "app-d",
			annotations: map[string]string{
				secretAllowedNamespacesAnnotation: "app-d-strongbox-shared,app-d-strongbox-shared-is-*",
			},
			wantErr: `secret "app-d/strongbox" cannot be used in namespace "app-d-strongbox-shared-not-allowed", the namespace must be listed in the 'kube-applier.io/allowed-namespaces' annotation`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			waybill := &kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-d",
					Namespace: tc.waybillNS,
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					AutoApply:                 ptr.To(true),
					StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox", Namespace: tc.secretNS},
				},
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "strongbox",
					Namespace:   tc.secretNS,
					Annotations: tc.annotations,
				},
			}

			err := checkSecretIsAllowed(waybill, secret)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}
