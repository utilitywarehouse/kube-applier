// Package webserver implements the Webserver struct which can serve the
// kube-applier status page and prometheus metrics, as well as receive run
// requests from users.
package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/sync/errgroup"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/run"
	"github.com/utilitywarehouse/kube-applier/sysutil"
	"github.com/utilitywarehouse/kube-applier/webserver/oidc"
)

const (
	defaultServerTemplatePath = "templates/status.html"
)

type KubeClient interface {
	ListWaybills(ctx context.Context) ([]kubeapplierv1alpha1.Waybill, error)
	HasAccess(ctx context.Context, waybill *kubeapplierv1alpha1.Waybill, email, verb string) (bool, error)
}

type Config struct {
	Authenticator        *oidc.Authenticator
	Clock                sysutil.ClockInterface
	DiffURLFormat        string
	KubeClient           KubeClient
	ListenPort           int
	RunQueue             chan<- run.Request
	StatusUpdateInterval time.Duration
	TemplatePath         string
	result               *Result
}

// WebServer struct
type WebServer struct {
	port                 int
	authenticator        *oidc.Authenticator
	clock                sysutil.ClockInterface
	statusPageTemplate   *template.Template
	result               *Result
	diffURLFormat        string
	kubeClient           KubeClient
	statusUpdateInterval time.Duration
	runQueue             chan<- run.Request
}

func New(cfg *Config) (*WebServer, error) {
	templatePath := cfg.TemplatePath
	if templatePath == "" {
		templatePath = defaultServerTemplatePath
	}

	statusPageTemplate, err := createTemplate(templatePath)
	if err != nil {
		return nil, err
	}

	return &WebServer{
		port:                 cfg.ListenPort,
		statusUpdateInterval: cfg.StatusUpdateInterval,
		runQueue:             cfg.RunQueue,
		authenticator:        cfg.Authenticator,
		clock:                cfg.Clock,
		statusPageTemplate:   statusPageTemplate,
		result:               cfg.result,
		kubeClient:           cfg.KubeClient,
		diffURLFormat:        cfg.DiffURLFormat,
	}, nil
}

// allStatusHandler serves a status page with info about the most recent applier run.
func (ws *WebServer) allStatusHandler(w http.ResponseWriter, r *http.Request) {
	if ws.authenticator != nil {
		_, err := ws.authenticator.Authenticate(r.Context(), w, r)
		if errors.Is(err, oidc.ErrRedirectRequired) {
			return
		}
		if err != nil {
			http.Error(w, "Error: Authentication failed", http.StatusInternalServerError)
			log.Logger("webserver").Error("Authentication failed", "error", err, "time", ws.clock.Now().String())
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(ws.result); err != nil {
		log.Logger("webserver").Error("Request failed", "error", http.StatusInternalServerError, "time", ws.clock.Now().String(), "err", err)
		panic(err)
	}
}

// namespaceStatusHandler serves a status page with info about the most recent applier run.
func (ws *WebServer) namespaceStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	if ws.authenticator != nil {
		_, err := ws.authenticator.Authenticate(r.Context(), w, r)
		if errors.Is(err, oidc.ErrRedirectRequired) {
			return
		}
		if err != nil {
			http.Error(w, "Error: Authentication failed", http.StatusInternalServerError)
			log.Logger("webserver").Error("Authentication failed", "error", err, "time", ws.clock.Now().String())
			return
		}
	}

	waybill := ws.result.Namespace(vars["namespace"])
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"Waybill":       waybill,
		"DiffURLFormat": ws.result.DiffURLFormat,
	}); err != nil {
		log.Logger("webserver").Error("Request failed", "error", http.StatusInternalServerError, "time", ws.clock.Now().String(), "err", err)
		panic(err)
	}
}

// handleForceRun serves an API endpoint for forcing a new run.
func (ws *WebServer) handleForceRun(w http.ResponseWriter, r *http.Request) {
	log.Logger("webserver").Info("Force run requested")
	var data struct {
		Result  string `json:"result"`
		Message string `json:"message"`
	}

	switch r.Method {
	case http.MethodPost:
		var (
			userEmail string
			err       error
		)
		if ws.authenticator != nil {
			userEmail, err = ws.authenticator.UserEmail(r.Context(), r)
			if err != nil {
				data.Result = "error"
				data.Message = "not authenticated"
				log.Logger("webserver").Error(data.Message, "error", err)
				w.WriteHeader(http.StatusForbidden)
				break
			}
		}

		if err := r.ParseForm(); err != nil {
			data.Result = "error"
			data.Message = "could not parse form data"
			log.Logger("webserver").Error(data.Message, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			break
		}

		ns := r.FormValue("namespace")
		if ns == "" {
			data.Result = "error"
			data.Message = "empty namespace value"
			log.Logger("webserver").Error(data.Message)
			w.WriteHeader(http.StatusBadRequest)
			break
		}

		waybills, err := ws.kubeClient.ListWaybills(r.Context())
		if err != nil {
			data.Result = "error"
			data.Message = "cannot list Waybills"
			log.Logger("webserver").Error(data.Message, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			break
		}

		var waybill *kubeapplierv1alpha1.Waybill
		for i := range waybills {
			if waybills[i].Namespace == ns {
				waybill = &waybills[i]
				break
			}
		}
		if waybill == nil {
			data.Result = "error"
			data.Message = fmt.Sprintf("cannot find Waybills in namespace '%s'", ns)
			w.WriteHeader(http.StatusBadRequest)
			break
		}

		if ws.authenticator != nil {
			// if the user can patch the Waybill, they are allowed to force a run
			hasAccess, err := ws.kubeClient.HasAccess(r.Context(), waybill, userEmail, "patch")
			if !hasAccess {
				data.Result = "error"
				data.Message = fmt.Sprintf("user %s is not allowed to force a run on waybill %s/%s", userEmail, waybill.Namespace, waybill.Name)
				if err != nil {
					log.Logger("webserver").Error(data.Message, "error", err)
				}
				w.WriteHeader(http.StatusForbidden)
				break
			}
		}

		run.Enqueue(ws.runQueue, run.ForcedRun, waybill)
		data.Result = "success"
		data.Message = "Run queued"
		w.WriteHeader(http.StatusOK)
	default:
		data.Result = "error"
		data.Message = "must be a POST request"
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "waybill/json; charset=UTF-8")
	json.NewEncoder(w).Encode(data)
}

// Start starts the webserver using the given port, and sets up handlers for:
// 1. Status page
// 2. Metrics
// 3. Static content
// 4. Endpoint for forcing a run
func (ws *WebServer) Start(ctx context.Context) error {
	log.Logger("webserver").Info("Launching")

	m := mux.NewRouter()
	addStatusEndpoints(m)
	m.PathPrefix("/api/v1/status/{namespace}").HandlerFunc(ws.namespaceStatusHandler)
	m.PathPrefix("/api/v1/status").HandlerFunc(ws.allStatusHandler)
	m.PathPrefix("/api/v1/forceRun").Methods(http.MethodPost).HandlerFunc(ws.handleForceRun)
	m.PathPrefix("/").Handler(http.FileServer(http.Dir("../static/build")))

	server := &http.Server{
		Addr:     fmt.Sprintf(":%v", ws.port),
		Handler:  cors.Default().Handler(m),
		ErrorLog: log.Logger("http.Server").StandardLogger(nil),
	}

	ws.result = &Result{
		Mutex:         &sync.Mutex{},
		DiffURLFormat: ws.diffURLFormat,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		ticker := time.NewTicker(ws.statusUpdateInterval)
		defer ticker.Stop()

		if err := ws.updateResult(ctx); err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if err := ws.updateResult(ctx); err != nil {
					return err
				}
			}
		}
	})

	g.Go(func() error {
		<-ctx.Done()
		return server.Shutdown(ctx)
	})

	g.Go(func() error {
		return server.ListenAndServe()
	})

	return g.Wait()
}

func (ws *WebServer) updateResult(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, ws.statusUpdateInterval-time.Second)
	defer cancel()
	waybills, err := ws.kubeClient.ListWaybills(ctx)
	if err != nil {
		return fmt.Errorf("Could not list Waybill resources: %v", err)
	}
	ws.result.Lock()
	ws.result.Waybills = waybills
	ws.result.Unlock()
	return nil
}
