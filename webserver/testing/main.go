package main

import (
	"context"
	"time"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/run"
	"github.com/utilitywarehouse/kube-applier/sysutil"
	"github.com/utilitywarehouse/kube-applier/webserver"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {

	ws, err := webserver.New(&webserver.Config{
		Authenticator:        nil,
		KubeClient:           &kubeApplier{},
		Clock:                &sysutil.Clock{},
		DiffURLFormat:        "",
		ListenPort:           8080,
		RunQueue:             make(chan<- run.Request, 100),
		StatusUpdateInterval: time.Second * 10,
		TemplatePath:         "../../templates/status.html",
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ws.Start(ctx); err != nil {
		panic(err)
	}

	if err := ws.Start(ctx); err != nil {
		panic(err)
	}

}

type kubeApplier struct{}

func (*kubeApplier) ListWaybills(ctx context.Context) ([]kubeapplierv1alpha1.Waybill, error) {
	t := true
	f := false
	return []kubeapplierv1alpha1.Waybill{
		{
			TypeMeta: v1.TypeMeta{
				Kind:       v1.FinalizerDeleteDependents,
				APIVersion: "1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "name",
				Namespace: "energy",
			},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				DryRun:      true,
				Prune:       &t,
				AutoApply:   &t,
				RunInterval: 3600,
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:      "/usr/local/bin/kustomize build /tmp/run_contact-channels_main_1622795682_repo_3566652641/dev-merit/contact-channels | /usr/local/bin/kubectl apply -f - --token=<omitted> -n contact-channels --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap --prune-whitelist=core/v1/Endpoints --prune-whitelist=core/v1/Event --prune-whitelist=core/v1/LimitRange --prune-whitelist=core/v1/PersistentVolumeClaim --prune-whitelist=core/v1/Pod --prune-whitelist=core/v1/PodTemplate --prune-whitelist=core/v1/ReplicationController --prune-whitelist=core/v1/ResourceQuota --prune-whitelist=core/v1/Secret --prune-whitelist=core/v1/ServiceAccount --prune-whitelist=core/v1/Service --prune-whitelist=apps/v1/DaemonSet --prune-whitelist=apps/v1/Deployment --prune-whitelist=apps/v1/ReplicaSet --prune-whitelist=apps/v1/StatefulSet --prune-whitelist=events.k8s.io/v1/Event --prune-whitelist=events.k8s.io/v1beta1/Event --prune-whitelist=autoscaling/v1/HorizontalPodAutoscaler --prune-whitelist=autoscaling/v2beta1/HorizontalPodAutoscaler --prune-whitelist=autoscaling/v2beta2/HorizontalPodAutoscaler --prune-whitelist=batch/v1/CronJob --prune-whitelist=batch/v1/Job --prune-whitelist=batch/v1beta1/CronJob --prune-whitelist=networking.k8s.io/v1/Ingress --prune-whitelist=networking.k8s.io/v1/NetworkPolicy --prune-whitelist=networking.k8s.io/v1beta1/Ingress --prune-whitelist=extensions/v1beta1/Ingress --prune-whitelist=policy/v1/PodDisruptionBudget --prune-whitelist=policy/v1beta1/PodDisruptionBudget --prune-whitelist=rbac.authorization.k8s.io/v1/RoleBinding --prune-whitelist=rbac.authorization.k8s.io/v1/Role --prune-whitelist=rbac.authorization.k8s.io/v1beta1/RoleBinding --prune-whitelist=rbac.authorization.k8s.io/v1beta1/Role --prune-whitelist=storage.k8s.io/v1beta1/CSIStorageCapacity --prune-whitelist=coordination.k8s.io/v1/Lease --prune-whitelist=coordination.k8s.io/v1beta1/Lease --prune-whitelist=discovery.k8s.io/v1/EndpointSlice --prune-whitelist=discovery.k8s.io/v1beta1/EndpointSlice --prune-whitelist=acme.cert-manager.io/v1/Challenge --prune-whitelist=acme.cert-manager.io/v1/Order --prune-whitelist=acme.cert-manager.io/v1beta1/Order --prune-whitelist=acme.cert-manager.io/v1beta1/Challenge --prune-whitelist=acme.cert-manager.io/v1alpha3/Challenge --prune-whitelist=acme.cert-manager.io/v1alpha3/Order --prune-whitelist=acme.cert-manager.io/v1alpha2/Challenge --prune-whitelist=acme.cert-manager.io/v1alpha2/Order --prune-whitelist=cert-manager.io/v1/Certificate --prune-whitelist=cert-manager.io/v1/Issuer --prune-whitelist=cert-manager.io/v1beta1/Certificate --prune-whitelist=cert-manager.io/v1beta1/Issuer --prune-whitelist=cert-manager.io/v1alpha3/Issuer --prune-whitelist=cert-manager.io/v1alpha3/Certificate --prune-whitelist=cert-manager.io/v1alpha2/Certificate --prune-whitelist=cert-manager.io/v1alpha2/Issuer --prune-whitelist=crd.projectcalico.org/v1/NetworkSet --prune-whitelist=crd.projectcalico.org/v1/NetworkPolicy --prune-whitelist=trident.netapp.io/v1/TridentSnapshot --prune-whitelist=trident.netapp.io/v1/TridentTransaction --prune-whitelist=trident.netapp.io/v1/TridentVersion --prune-whitelist=trident.netapp.io/v1/TridentBackendConfig --prune-whitelist=trident.netapp.io/v1/TridentNode --prune-whitelist=trident.netapp.io/v1/TridentVolume --prune-whitelist=trident.netapp.io/v1/TridentBackend --prune-whitelist=trident.netapp.io/v1/TridentStorageClass --prune-whitelist=volumesnapshot.external-storage.k8s.io/v1/VolumeSnapshot --prune-whitelist=argoproj.io/v1alpha1/Application --prune-whitelist=argoproj.io/v1alpha1/AppProject --prune-whitelist=config.gatekeeper.sh/v1alpha1/Config --prune-whitelist=kube-applier.io/v1alpha1/Waybill --prune-whitelist=traefik.containo.us/v1alpha1/IngressRouteUDP --prune-whitelist=traefik.containo.us/v1alpha1/TLSStore --prune-whitelist=traefik.containo.us/v1alpha1/TLSOption --prune-whitelist=traefik.containo.us/v1alpha1/Middleware --prune-whitelist=traefik.containo.us/v1alpha1/TraefikService --prune-whitelist=traefik.containo.us/v1alpha1/IngressRoute --prune-whitelist=traefik.containo.us/v1alpha1/ServersTransport --prune-whitelist=traefik.containo.us/v1alpha1/IngressRouteTCP --prune-whitelist=status.gatekeeper.sh/v1beta1/ConstraintTemplatePodStatus --prune-whitelist=status.gatekeeper.sh/v1beta1/ConstraintPodStatus",
					Commit:       "5f3fd1cc",
					ErrorMessage: "error",
					Finished:     v1.NewTime(time.Now()),
					Output:       "hello",
					Started:      v1.Now(),
					Success:      true,
					Type:         "Scheduled run",
				},
			},
		},
		{
			TypeMeta: v1.TypeMeta{
				Kind:       v1.FinalizerDeleteDependents,
				APIVersion: "1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "name",
				Namespace: "account-platform",
			},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   &f,
				DryRun:      false,
				Prune:       &t,
				RunInterval: 3600,
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:      "command",
					Commit:       "commit",
					ErrorMessage: "error",
					Finished:     v1.NewTime(time.Now()),
					Output:       "hello",
					Started:      v1.Now(),
					Success:      false,
					Type:         "Force run",
				},
			},
		},
	}, nil
}

func (*kubeApplier) HasAccess(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, email, verb string) (bool, error) {
	return false, nil
}
