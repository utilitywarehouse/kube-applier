package kube

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/utilitywarehouse/kube-applier/metrics"
	"github.com/utilitywarehouse/kube-applier/sysutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	// in case of local kube config
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// Default location of the service-account token on the cluster
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// Location of the kubeconfig template file within the container - see ADD command in Dockerfile
	kubeconfigTemplatePath = "/templates/kubeconfig"

	// Location of the written kubeconfig file within the container
	kubeconfigFilePath = "/etc/kubeconfig"

	enabledAnnotation = "kube-applier.io/enabled"
	dryRunAnnotation  = "kube-applier.io/dry-run"
	pruneAnnotation   = "kube-applier.io/prune"
)

// To make testing possible
var execCommand = exec.Command

//todo(catalin-ilea) Add core/v1/Secret when we plug in strongbox
var pruneWhitelist = []string{
	"apps/v1/DaemonSet",
	"apps/v1/Deployment",
	"apps/v1/StatefulSet",
	"autoscaling/v1/HorizontalPodAutoscaler",
	"batch/v1/Job",
	"core/v1/ConfigMap",
	"core/v1/Pod",
	"core/v1/Service",
	"core/v1/ServiceAccount",
	"networking.k8s.io/v1beta1/Ingress",
	"networking.k8s.io/v1/NetworkPolicy",
}

// KAAnnotations contains the standard set of annotations on the Namespace
// resource defining behaviour for that Namespace
type KAAnnotations struct {
	Enabled string
	DryRun  string
	Prune   string
}

// ClientInterface allows for mocking out the functionality of Client when testing the full process of an apply run.
type ClientInterface interface {
	Apply(path, namespace string, dryRun, prune, kustomize bool) (string, string, error)
	NamespaceAnnotations(namespace string) (KAAnnotations, error)
}

// Client enables communication with the Kubernetes API Server through kubectl commands.
// The Server field enables discovery of the API server when kube-proxy is not configured (see README.md for more information).
type Client struct {
	Server    string
	Label     string
	Metrics   metrics.PrometheusInterface
	NsWatcher *namespaceWatcher
}

// Configure writes the kubeconfig file to be used for authenticating kubectl commands and to watch namespaces.
func (c *Client) Configure() error {
	// No need to write a kubeconfig file if Server is not specified (API server will be discovered via kube-proxy).
	if c.Server == "" {
		return nil
	}

	f, err := os.Create(kubeconfigFilePath)
	if err != nil {
		return errors.Wrap(err, "creating kubeconfig file failed")
	}
	defer f.Close()

	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return errors.Wrap(err, "cannot access token for kubeconfig file")
	}

	var data struct {
		Token  string
		Server string
	}
	data.Token = string(token)
	data.Server = c.Server

	template, err := sysutil.CreateTemplate(kubeconfigTemplatePath)
	if err != nil {
		return errors.Wrap(err, "parsing kubeconfig template failed")
	}
	if err := template.Execute(f, data); err != nil {
		return errors.Wrap(err, "applying kubeconfig template failed")
	}

	return nil
}

// StartWatching creates a namespace watcher and starts it up
func (c *Client) StartWatching() error {

	// create a kube client to pass to the watcher
	// If server is set we should expect config under kubeconfigFilePath
	// else try to use in cluster config
	var config *rest.Config
	if c.Server != "" {
		config, _ = clientcmd.BuildConfigFromFlags(
			"", kubeconfigFilePath)
	} else {
		config, _ = rest.InClusterConfig()
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// resyncPeriod 0 means getting updates only on events and not also on
	// static intervals
	c.NsWatcher = newNamespaceWatcher(kubeClient, 0, c.Metrics)

	// Start watching namespaces. ns watch start will block until stop is
	// called so spawn a new routine for it
	go c.NsWatcher.Start()

	return nil
}

// Apply attempts to "kubectl apply" the files located at path. It returns the
// full apply command and its output.
//
// kustomize - Do a `kustomize build | kubectl apply -f -` on the path, set to if there is a
//             `kustomization.yaml` found in the path
func (c *Client) Apply(path, namespace string, dryRun, prune, kustomize bool) (string, string, error) {
	var args []string

	if kustomize {
		args = []string{"kubectl", "apply", fmt.Sprintf("--server-dry-run=%t", dryRun), "-f", "-", "-n", namespace}
	} else {
		args = []string{"kubectl", "apply", fmt.Sprintf("--server-dry-run=%t", dryRun), "-R", "-f", path, "-n", namespace}
	}

	if prune {
		args = append(args, "--prune")
		args = append(args, "--all")
		for _, w := range pruneWhitelist {
			args = append(args, "--prune-whitelist="+w)
		}
	}

	if c.Server != "" {
		args = append(args, fmt.Sprintf("--kubeconfig=%s", kubeconfigFilePath))
	}

	kubectlCmd := exec.Command(args[0], args[1:]...)

	var cmdStr string
	if kustomize {
		cmdStr = "kustomize build " + path + " | " + strings.Join(args, " ")
		kustomizeCmd := exec.Command("kustomize", "build", path)
		pipe, err := kustomizeCmd.StdoutPipe()
		if err != nil {
			return cmdStr, "", err
		}
		kubectlCmd.Stdin = pipe

		err = kustomizeCmd.Start()
		if err != nil {
			fmt.Printf("%s", err)
			return cmdStr, "", err
		}
	} else {
		cmdStr = strings.Join(args, " ")
	}

	out, err := kubectlCmd.CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
		}
		return cmdStr, string(out), err
	}
	c.Metrics.UpdateKubectlExitCodeCount(path, 0)

	return cmdStr, string(out), err
}

// NamespaceAnnotations returns string values of kube-applier annotaions
func (c *Client) NamespaceAnnotations(namespace string) (KAAnnotations, error) {
	kaa := KAAnnotations{}

	ns, err := c.NsWatcher.Get(namespace)
	if err != nil {
		return kaa, err
	}

	kaa.Enabled = ns.Annotations[enabledAnnotation]
	kaa.DryRun = ns.Annotations[dryRunAnnotation]
	kaa.Prune = ns.Annotations[pruneAnnotation]

	return kaa, nil
}
