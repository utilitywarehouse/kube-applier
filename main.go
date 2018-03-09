package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
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
)

func main() {
	app := cli.App(webserver.AppName, webserver.AppDescription)
	repoPath := app.String(cli.StringOpt{
		Name:   "repo-path",
		Desc:   "Git repo path",
		EnvVar: "REPO_PATH",
	})
	listenPort := app.Int(cli.IntOpt{
		Name:   "listenport",
		Value:  8080,
		Desc:   "Listen port",
		EnvVar: "LISTEN_PORT",
	})
	server := app.String(cli.StringOpt{
		Name:   "server",
		Value:  "",
		Desc:   "K8s server. Mainly for local testing.",
		EnvVar: "SERVER",
	})
	diffURLFormat := app.String(cli.StringOpt{
		Name:   "diff-url-format",
		Value:  "https://github.com/utilitywarehouse/kubernetes-manifests/commit/%s",
		Desc:   "Github commit diff url",
		EnvVar: "DIFF_URL_FORMAT",
	})
	pollInterval := app.Int(cli.IntOpt{
		Name:   "poll-interval-seconds",
		Value:  5,
		Desc:   "Poll interval",
		EnvVar: "POLL_INTERVAL_SECONDS",
	})
	fullRunInterval := app.Int(cli.IntOpt{
		Name:   "full-run-interval-seconds",
		Value:  60,
		Desc:   "Full run interval",
		EnvVar: "FULL_RUN_INTERVAL_SECONDS",
	})
	dryRun := app.Bool(cli.BoolOpt{
		Name:   "dry-run",
		Value:  false,
		Desc:   "Dry run",
		EnvVar: "DRY_RUN",
	})
	prune := app.Bool(cli.BoolOpt{
		Name:   "prune",
		Value:  true,
		Desc:   "kubectl --prune flag used when applying manifests. Default true",
		EnvVar: "KUBE_PRUNE",
	})
	strictApply := app.Bool(cli.BoolOpt{
		Name:   "strict-apply",
		Value:  false,
		Desc:   "Use kube-applier service-accounts for every namespace",
		EnvVar: "STRICT_APPLY",
	})
	label := app.String(cli.StringOpt{
		Name:   "label",
		Value:  "automaticDeployment",
		Desc:   "K8s label used to enable/disable automatic deployments.",
		EnvVar: "LABEL",
	})
	logLevel := app.String(cli.StringOpt{
		Name:   "log",
		Value:  "warn",
		Desc:   "Log level [trace|debug|info|warn|error] case insensitive",
		EnvVar: "LOG_LEVEL",
	})

	log.InitLogger(*logLevel)

	if *diffURLFormat != "" && !strings.Contains(*diffURLFormat, "%s") {
		log.Logger.Error(fmt.Sprintf("Invalid DIFF_URL_FORMAT, must contain %q: %v", "%s", *diffURLFormat))
		os.Exit(1)

	}
	app.Action = func() {
		metrics := &metrics.Prometheus{}
		metrics.Init()

		log.Logger.StartMetrics(metrics)

		clock := &sysutil.Clock{}

		if err := sysutil.WaitForDir(*repoPath, clock, waitForRepoInterval); err != nil {
			log.Logger.Error("error", err)
			os.Exit(1)
		}

		kubeClient := &kube.Client{Server: *server, Label: *label}
		if err := kubeClient.Configure(); err != nil {
			log.Logger.Error("kubectl configuration failed", "error", err)
		}

		batchApplier := &run.BatchApplier{
			KubeClient:  kubeClient,
			DryRun:      *dryRun,
			Prune:       *prune,
			StrictApply: *strictApply,
			Metrics:     metrics,
		}

		gitUtil := &git.GitUtil{
			RepoPath: *repoPath,
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

		runner := &run.Runner{
			RepoPath:      *repoPath,
			BatchApplier:  batchApplier,
			GitUtil:       gitUtil,
			Clock:         clock,
			Metrics:       metrics,
			DiffURLFormat: *diffURLFormat,
			RunQueue:      runQueue,
			RunResults:    runResults,
			Errors:        errors,
		}
		scheduler := &run.Scheduler{
			GitUtil:         gitUtil,
			PollInterval:    time.Duration(*pollInterval) * time.Second,
			FullRunInterval: time.Duration(*fullRunInterval) * time.Second,
			RunQueue:        runQueue,
			Errors:          errors,
		}
		webserver := &webserver.WebServer{
			ListenPort: *listenPort,
			Clock:      clock,
			RunQueue:   runQueue,
			RunResults: runResults,
			Errors:     errors,
		}

		go scheduler.Start()
		go runner.Start()
		go webserver.Start()

		for err := range errors {
			log.Logger.Error("error", err)
			os.Exit(1)
		}
	}
	app.Run(os.Args)
}
