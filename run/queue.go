package run

import (
	"fmt"
	"sync"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

// Queue holds run requests keyed by Waybill. Requests for a Waybill that is
// already waiting are merged into the pending one, and a Waybill being
// processed is re-queued at most once, so a Waybill is never applied by two
// workers at the same time.
type Queue struct {
	workQueue workqueue.TypedInterface[string]
	// pending holds the request data for each key in workQueue, because the
	// work queue only stores the Waybill's namespace/name.
	mutex   sync.Mutex
	pending map[string]Request
}

// NewQueue returns an empty run queue.
func NewQueue() *Queue {
	return &Queue{
		workQueue: workqueue.NewTyped[string](),
		pending:   make(map[string]Request),
	}
}

func requestKey(waybill *kubeapplierv1alpha1.Waybill) string {
	return fmt.Sprintf("%s/%s", waybill.Namespace, waybill.Name)
}

// Enqueue adds a run request for the Waybill. If a request for the same
// Waybill is already waiting it is merged into that one, with the newer
// Waybill and a ForcedRun type taking precedence.
func (q *Queue) Enqueue(t Type, waybill *kubeapplierv1alpha1.Waybill) {
	wbId := requestKey(waybill)
	if t != ForcedRun && !ptr.Deref(waybill.Spec.AutoApply, true) {
		log.Logger("runner").Debug("Run ignored, waybill autoApply is disabled", "waybill", wbId, "type", t)
		return
	}
	// DeepCopy so workers never mutate the scheduler's cached Waybill.
	req := Request{Type: t, Waybill: waybill.DeepCopy()}

	q.mutex.Lock()
	if existing, ok := q.pending[wbId]; ok {
		existing.Waybill = req.Waybill
		if t == ForcedRun && existing.Type != ForcedRun {
			// The pending request is about to run as a ForcedRun, so
			// keep the queued gauge attributed to its actual type.
			metrics.UpdateRunRequest(existing.Type.String(), existing.Waybill, -1)
			metrics.UpdateRunRequest(ForcedRun.String(), existing.Waybill, 1)
			existing.Type = ForcedRun
		}
		q.pending[wbId] = existing
		q.mutex.Unlock()
		metrics.AddRunRequestQueueDeduplicated(t.String(), waybill)
		log.Logger("runner").Debug("Run request merged into pending run", "waybill", wbId, "type", t)
		return
	}
	q.pending[wbId] = req
	q.mutex.Unlock()

	q.workQueue.Add(wbId)
	metrics.UpdateRunRequest(t.String(), waybill, 1)
	log.Logger("runner").Debug("Run queued", "waybill", wbId, "type", t)
}

// Get blocks until a request is available and consumes its pending entry, so
// an Enqueue for the same Waybill while it is being processed creates a fresh
// entry for the follow-up run. The returned key must be passed to Done once
// the request has been handled. It returns shutdown=true when the queue has
// been shut down and contains no more requests.
func (q *Queue) Get() (request Request, key string, shutdown bool) {
	key, shutdown = q.workQueue.Get()
	if shutdown {
		return Request{}, "", true
	}
	return q.take(key), key, false
}

// Done marks the request identified by key as processed, re-queueing it if it
// was enqueued again in the meantime.
func (q *Queue) Done(key string) {
	q.workQueue.Done(key)
}

// ShutDown stops accepting new requests. Requests already queued are still
// returned by Get until the queue is empty.
func (q *Queue) ShutDown() {
	q.workQueue.ShutDown()
}

func (q *Queue) take(key string) Request {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	req, ok := q.pending[key]
	if !ok {
		// Unreachable: a key is only added to workQueue after its
		// pending entry is written, and entries are only removed here.
		log.Logger("runner").Error("No pending request for queued Waybill", "waybill", key)
		return Request{}
	}
	delete(q.pending, key)
	return req
}
