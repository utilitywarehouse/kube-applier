package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
)

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
