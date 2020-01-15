package kube

import (
	"fmt"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/pkg/errors"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

type namespaceWatcher struct {
	client       kubernetes.Interface
	resyncPeriod time.Duration
	stopChannel  chan struct{}
	store        cache.Store
	Metrics      metrics.PrometheusInterface
}

func newNamespaceWatcher(client kubernetes.Interface, resyncPeriod time.Duration, metrics metrics.PrometheusInterface) *namespaceWatcher {
	return &namespaceWatcher{
		client:       client,
		resyncPeriod: resyncPeriod,
		stopChannel:  make(chan struct{}),
		Metrics:      metrics,
	}
}

func (nw *namespaceWatcher) updateNamespaceMetrics(ns *v1.Namespace) {
	name := ns.Name

	// Enabled
	anno, ok := ns.Annotations[enabledAnnotation]
	if !ok {
		// Try to delete the metric since annotation is not found
		nw.Metrics.DeleteEnabled(name)
	} else {
		// Parse annotation and update metric, set false if parsing errors
		enabled, err := strconv.ParseBool(anno)
		if err != nil {
			log.Logger.Warn(
				"Error parsing namespace annotation kube-applier.io/enabled, setting metric to false",
				"namespace", name,
				"error", err,
			)
			enabled = false

		}
		nw.Metrics.UpdateEnabled(name, enabled)
	}

	// DryRun
	anno, ok = ns.Annotations[dryRunAnnotation]
	if !ok {
		// Try to delete the metric since annotation is not found
		nw.Metrics.DeleteDryRun(name)
	} else {
		// Parse annotation and update metric, set false if parsing errors
		dryRun, err := strconv.ParseBool(anno)
		if err != nil {
			log.Logger.Warn("Error parsing namespace annotation kube-applier.io/dryRun, setting metric to false",
				"namespace", name,
				"error", err,
			)
			dryRun = false
		}
		nw.Metrics.UpdateDryRun(name, dryRun)
	}

	// Prune
	anno, ok = ns.Annotations[pruneAnnotation]
	if !ok {
		// Try to delete the metric since annotation is not found
		nw.Metrics.DeletePrune(name)
	} else {
		prune, err := strconv.ParseBool(anno)
		if err != nil {
			log.Logger.Warn("Error parsing namespace annotation kube-applier.io/prune, setting metric to false",
				"namespace", name,
				"error", err,
			)
			prune = false
		}
		nw.Metrics.UpdatePrune(name, prune)
	}
}

func (nw *namespaceWatcher) deleteNamespaceMetrics(ns *v1.Namespace) {
	name := ns.Name
	nw.Metrics.DeleteEnabled(name)
	nw.Metrics.DeleteDryRun(name)
	nw.Metrics.DeletePrune(name)
}

func (nw *namespaceWatcher) eventHandler(eventType watch.EventType, old *v1.Namespace, new *v1.Namespace) {
	switch eventType {
	case watch.Added:
		nw.updateNamespaceMetrics(new)
	case watch.Modified:
		nw.updateNamespaceMetrics(new)
	case watch.Deleted:
		nw.deleteNamespaceMetrics(new)
	default:
		log.Logger.Info("Unknown namespace event received", eventType)
	}
}

func (nw *namespaceWatcher) Start() {
	listWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return nw.client.CoreV1().Namespaces().List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return nw.client.CoreV1().Namespaces().Watch(options)
		},
	}
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nw.eventHandler(watch.Added, nil, obj.(*v1.Namespace))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			nw.eventHandler(watch.Modified, oldObj.(*v1.Namespace), newObj.(*v1.Namespace))
		},
		DeleteFunc: func(obj interface{}) {
			nw.eventHandler(watch.Deleted, obj.(*v1.Namespace), nil)
		},
	}
	store, controller := cache.NewInformer(listWatch, &v1.Namespace{}, nw.resyncPeriod, eventHandler)
	nw.store = store
	log.Logger.Info("Starting namespace watcher")
	// Running controller will block until writing on the stop channel.
	controller.Run(nw.stopChannel)
	log.Logger.Info("Stopped namespace watcher")
}

func (nw *namespaceWatcher) Stop() {
	log.Logger.Info("Stopping namespace watcher...")
	close(nw.stopChannel)
}

func (nw *namespaceWatcher) Get(namespace string) (*v1.Namespace, error) {
	ns, exists, err := nw.store.GetByKey(namespace)
	if exists {
		return ns.(*v1.Namespace), err
	}
	return &v1.Namespace{}, errors.New(
		fmt.Sprintf("namespace %s does not exist", namespace),
	)
}
