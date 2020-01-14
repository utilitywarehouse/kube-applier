package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/utilitywarehouse/kube-applier/git"
	"github.com/utilitywarehouse/kube-applier/kube"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/metrics"
	"github.com/utilitywarehouse/kube-applier/run"
	"github.com/utilitywarehouse/kube-applier/sysutil"
	"github.com/utilitywarehouse/kube-applier/webserver"
)

const (
	// Number of seconds to wait in between attempts to locate the repo at the specified path.
	// Git-sync atomically places the repo at the specified path once it is finished pulling, so it will not be present immediately.
	waitForRepoInterval = 1 * time.Second
	waitForRepoTimeout  = 120 * time.Second
)

var (
	repoPath        = os.Getenv("REPO_PATH")
	repoPathFilters = os.Getenv("REPO_PATH_FILTERS")
	listenPort      = os.Getenv("LISTEN_PORT")
	pollInterval    = os.Getenv("POLL_INTERVAL_SECONDS")
	fullRunInterval = os.Getenv("FULL_RUN_INTERVAL_SECONDS")
	dryRun          = os.Getenv("DRY_RUN")
	logLevel        = os.Getenv("LOG_LEVEL")

	// kube server. Mainly for local testing.
	server = os.Getenv("SERVER")

	// Github commit diff url
	diffURLFormat = os.Getenv("DIFF_URL_FORMAT")
)

func validate() {
	if repoPath == "" {
		fmt.Println("Need to export REPO_PATH")
		os.Exit(1)
	}

	if listenPort == "" {
		listenPort = "8080"
	} else {
		_, err := strconv.Atoi(listenPort)
		if err != nil {
			fmt.Println("LISTEN_PORT must be an int")
			os.Exit(1)
		}
	}

	if diffURLFormat != "" && !strings.Contains(diffURLFormat, "%s") {
		fmt.Printf("Invalid DIFF_URL_FORMAT, must contain %q: %v\n", "%s", diffURLFormat)
		os.Exit(1)
	}

	if pollInterval == "" {
		pollInterval = "5"
	} else {
		_, err := strconv.Atoi(pollInterval)
		if err != nil {
			fmt.Println("POLL_INTERVAL_SECONDS must be an int")
			os.Exit(1)
		}
	}

	if fullRunInterval == "" {
		fullRunInterval = "3600"
	} else {
		_, err := strconv.Atoi(fullRunInterval)
		if err != nil {
			fmt.Println("FULL_RUN_INTERVAL_SECONDS must be an int")
			os.Exit(1)
		}
	}

	if dryRun == "" {
		dryRun = "false"
	} else {
		_, err := strconv.ParseBool(dryRun)
		if err != nil {
			fmt.Println("DRY_RUN must be a boolean")
			os.Exit(1)
		}
	}

	// log level [trace|debug|info|warn|error] case insensitive
	if logLevel == "" {
		logLevel = "warn"
	}
}

func main() {
	validate()

	log.InitLogger(logLevel)

	metrics := &metrics.Prometheus{}
	metrics.Init()

	clock := &sysutil.Clock{}

	if err := sysutil.WaitForDir(repoPath, waitForRepoInterval, waitForRepoTimeout); err != nil {
		log.Logger.Error("error", err)
		os.Exit(1)
	}

	kubeClient := &kube.Client{
		Server:  server,
		Metrics: metrics,
	}

	if err := kubeClient.Configure(); err != nil {
		log.Logger.Error("kubectl configuration failed", "error", err)
	}

	if err := kubeClient.StartWatching(); err != nil {
		log.Logger.Error("failed to start the namespace watcher", "error", err)
	}

	dr, _ := strconv.ParseBool(dryRun)
	batchApplier := &run.BatchApplier{
		KubeClient: kubeClient,
		DryRun:     dr,
		Metrics:    metrics,
	}

	gitUtil := &git.Util{
		RepoPath: repoPath,
	}

	// Webserver and scheduler send run requests to runQueue channel, runner receives the requests and initiates runs.
	// Only 1 pending request may sit in the queue at a time.
	runQueue := make(chan bool, 1)

	// Runner sends run results to runResults channel, webserver receives the results and displays them.
	// Limit of 5 is arbitrary - there is significant delay between sends, and receives are handled near instantaneously.
	runResults := make(chan run.Result, 5)

	// Runner, webserver, and scheduler all send fatal errors to errors channel, and main() exits upon receiving an error.
	// No limit needed, as a single fatal error will exit the program anyway.
	errors := make(chan error)

	var repoPathFiltersSlice []string
	if repoPathFilters != "" {
		repoPathFiltersSlice = strings.Split(repoPathFilters, ",")
	}

	runner := &run.Runner{
		RepoPath:        repoPath,
		RepoPathFilters: repoPathFiltersSlice,
		BatchApplier:    batchApplier,
		GitUtil:         gitUtil,
		Clock:           clock,
		Metrics:         metrics,
		DiffURLFormat:   diffURLFormat,
		RunQueue:        runQueue,
		RunResults:      runResults,
		Errors:          errors,
	}

	pi, _ := strconv.Atoi(pollInterval)
	fi, _ := strconv.Atoi(fullRunInterval)
	scheduler := &run.Scheduler{
		GitUtil:         gitUtil,
		PollInterval:    time.Duration(pi) * time.Second,
		FullRunInterval: time.Duration(fi) * time.Second,
		RepoPathFilters: repoPathFiltersSlice,
		RunQueue:        runQueue,
		Errors:          errors,
	}

	lp, _ := strconv.Atoi(listenPort)
	webserver := &webserver.WebServer{
		ListenPort: lp,
		Clock:      clock,
		RunQueue:   runQueue,
		RunResults: runResults,
		Errors:     errors,
	}

	go scheduler.Start()
	go runner.Start()
	go webserver.Start()

	err := <-errors
	log.Logger.Error("Fatal error, exiting", "error", err)
	os.Exit(1)
}
