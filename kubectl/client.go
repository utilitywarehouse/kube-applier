package kubectl

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/utilitywarehouse/kube-applier/metrics"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

var (
	// To make testing possible
	execCommand          = exec.Command
	omitErrOutputMessage = "Some error output has been omitted because it may contain sensitive data\n"
)

// ClientInterface allows for mocking out the functionality of Client when testing the full process of an apply run.
type ClientInterface interface {
	Apply(path, namespace, dryRunStrategy string, kustomize bool, pruneWhitelist []string) (string, string, error)
}

// Client enables communication with the Kubernetes API Server through kubectl commands.
type Client struct {
	Label   string
	Metrics metrics.PrometheusInterface
}

// Apply attempts to "kubectl apply" the files located at path. It returns the
// full apply command and its output.
func (c *Client) Apply(path, namespace, dryRunStrategy string, kustomize bool, pruneWhitelist []string) (string, string, error) {
	if kustomize {
		return c.applyKustomize(path, namespace, dryRunStrategy, pruneWhitelist)
	}
	return c.apply(path, namespace, dryRunStrategy, pruneWhitelist)
}

// apply runs `kubectl apply -f <path>`
func (c *Client) apply(path, namespace, dryRunStrategy string, pruneWhitelist []string) (string, string, error) {
	args := []string{"kubectl", "apply", fmt.Sprintf("--dry-run=%s", dryRunStrategy), "-R", "-f", path, "-n", namespace}
	args = pruneArgs(args, pruneWhitelist)

	kubectlCmd := exec.Command(args[0], args[1:]...)
	out, err := kubectlCmd.CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
		}
		// Filter potential secret leaks out of the output
		return kubectlCmd.String(), filterErrOutput(string(out)), err
	}
	c.Metrics.UpdateKubectlExitCodeCount(path, 0)

	return kubectlCmd.String(), string(out), nil
}

// applyKustomize does a `kustomize build | kubectl apply -f -` on the path
func (c *Client) applyKustomize(path, namespace, dryRunStrategy string, pruneWhitelist []string) (string, string, error) {
	var kustomizeStdout, kustomizeStderr bytes.Buffer

	kustomizeCmd := exec.Command("kustomize", "build", path)
	kustomizeCmd.Stdout = &kustomizeStdout
	kustomizeCmd.Stderr = &kustomizeStderr

	err := kustomizeCmd.Run()
	if err != nil {
		return kustomizeCmd.String(), kustomizeStderr.String(), err
	}

	// Split the stdout into secrets and other resources
	stdout, err := ioutil.ReadAll(&kustomizeStdout)
	if err != nil {
		return kustomizeCmd.String(), "error reading kustomize output", err
	}
	resources, secrets, err := splitSecrets(stdout)
	if err != nil {
		return kustomizeCmd.String(), "error extracting secrets from kustomize output", err
	}
	if len(resources) == 0 && len(secrets) == 0 {
		return kustomizeCmd.String(), "", fmt.Errorf("No resources were extracted from the kustomize output")
	}

	args := []string{"kubectl", "apply", fmt.Sprintf("--dry-run=%s", dryRunStrategy), "-f", "-", "-n", namespace}

	// This is the command we are effectively applying. In actuality we're splitting it into two
	// separate invocations of kubectl but we'll return this as the command
	// string.
	displayArgs := pruneArgs(args, pruneWhitelist)
	kubectlCmd := exec.Command(displayArgs[0], displayArgs[1:]...)
	cmdStr := kustomizeCmd.String() + " | " + kubectlCmd.String()

	var kubectlOut []byte

	if len(resources) > 0 {
		resourcesArgs := args

		// Don't prune secrets
		resourcesPruneWhitelist := []string{}
		for _, w := range pruneWhitelist {
			if w != "core/v1/Secret" {
				resourcesPruneWhitelist = append(resourcesPruneWhitelist, w)
			}
		}
		resourcesArgs = pruneArgs(resourcesArgs, resourcesPruneWhitelist)

		resourcesKubectlCmd := exec.Command(resourcesArgs[0], resourcesArgs[1:]...)
		resourcesKubectlCmd.Stdin = bytes.NewReader(resources)

		out, err := resourcesKubectlCmd.CombinedOutput()
		kubectlOut = append(kubectlOut, out...)
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
			}
			return cmdStr, string(kubectlOut), err
		}
		c.Metrics.UpdateKubectlExitCodeCount(path, 0)
	}

	if len(secrets) > 0 {
		secretsArgs := args

		// Only prune secrets
		var secretsPruneWhitelist []string
		for _, w := range pruneWhitelist {
			if w == "core/v1/Secret" {
				secretsPruneWhitelist = append(secretsPruneWhitelist, w)
			}
		}
		secretsArgs = pruneArgs(secretsArgs, secretsPruneWhitelist)

		secretsKubectlCmd := exec.Command(secretsArgs[0], secretsArgs[1:]...)
		secretsKubectlCmd.Stdin = bytes.NewReader(secrets)

		out, err := secretsKubectlCmd.CombinedOutput()
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
			}
			// Don't append the actual output, as the error output
			// from kubectl can leak the content of secrets
			kubectlOut = append(kubectlOut, []byte(omitErrOutputMessage)...)
			return cmdStr, string(kubectlOut), err
		}
		c.Metrics.UpdateKubectlExitCodeCount(path, 0)
		kubectlOut = append(kubectlOut, out...)
	}

	return cmdStr, string(kubectlOut), nil
}

// pruneArgs appends prune arguments to a list of arguments
func pruneArgs(args []string, pruneWhitelist []string) []string {
	if len(pruneWhitelist) > 0 {
		args = append(args, []string{"--prune", "--all"}...)
		for _, w := range pruneWhitelist {
			args = append(args, "--prune-whitelist="+w)
		}
	}

	return args
}

// filterErrOutput squashes output that may contain potential leaked secrets
func filterErrOutput(out string) string {
	if strings.Contains(out, "Secret") || strings.Contains(out, "base64") {
		return omitErrOutputMessage
	}

	return out
}

// splitSecrets will take a yaml file and separate the resources into Secrets
// and other resources. This allows Secrets to be applied separately to other
// resources.
func splitSecrets(yamlData []byte) (resources, secrets []byte, err error) {
	objs, err := splitYAML(yamlData)
	if err != nil {
		return resources, secrets, err
	}

	secretsDocs := [][]byte{}
	resourcesDocs := [][]byte{}
	for _, obj := range objs {
		y, err := yaml.Marshal(obj)
		if err != nil {
			return resources, secrets, err
		}
		if obj.Object["kind"] == "Secret" {
			secretsDocs = append(secretsDocs, y)
		} else {
			resourcesDocs = append(resourcesDocs, y)
		}
	}

	secrets = bytes.Join(secretsDocs, []byte("---\n"))
	resources = bytes.Join(resourcesDocs, []byte("---\n"))

	return resources, secrets, nil
}

// splitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
//
// Taken from the gitops-engine:
//  - https://github.com/argoproj/gitops-engine/blob/cc0fb5531c29c193291a7f97a50921f544b2d3b9/pkg/utils/kube/kube.go#L282-L310
func splitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []*unstructured.Unstructured
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(ext.Raw, u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}
