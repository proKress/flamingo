package main

import (
	"context"
	"fmt"
	"github.com/prokress/flamingo/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"text/tabwriter"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get Flamingo applications",
	Long: `Get Flamingo applications

# List all Flamingo applications in the given namespace
flamingo get --namespace=default
NAMESPACE       APP       APP-TYPE     REVISION                SUSPENDED       READY   MESSAGE                                  
default         podinfo   kustomize    latest@sha256:3f432793  False           True    Applied revision: latest@sha256:3f432793
`,
	RunE: getCmdRun,
}

var getCmdFlags struct {
	all bool
}

func init() {
	getCmd.Flags().BoolVarP(&getCmdFlags.all, "all-namespaces", "A", false, "list all Flamingo applications in all namespaces")

	rootCmd.AddCommand(getCmd)
}

func getCmdRun(cmd *cobra.Command, args []string) error {
	cli, err := utils.KubeClient(kubeconfigArgs, kubeclientOptions)
	if err != nil {
		return err
	}

	gvk := schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	}

	namespace := *kubeconfigArgs.Namespace
	labelSelector := map[string]string{
		"app.kubernetes.io/managed-by":   "flamingo",
		"flamingo/destination-namespace": namespace,
	}
	if getCmdFlags.all {
		delete(labelSelector, "flamingo/destination-namespace")
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)
	if err := cli.List(context.TODO(), list, client.InNamespace(rootArgs.applicationNamespace), client.MatchingLabels(labelSelector)); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tAPP-NS\tAPP\tFLUX-TYPE\tSOURCE-TYPE\tCLUSTER\tSTATUS\tMESSAGE")
	for _, item := range list.Items {
		labels := item.GetLabels()
		// Extract the necessary fields from the Unstructured object
		// This is just an example, you'll need to adjust based on the actual structure of your Argo CD objects
		appType := labels["flamingo/workload-type"]
		sourceType := labels["flamingo/source-type"]
		objectNs := labels["flamingo/destination-namespace"]
		clusterName := labels["flamingo/cluster-name"]
		status, err := extractStatus(&item)
		if err != nil {
			status = err.Error()
		}
		message, err := extractMessage(&item)
		if err != nil {
			message = err.Error()
		}
		if len(message) > 40 {
			message = message[:40] + " ..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			objectNs,
			item.GetNamespace(),
			item.GetName(),
			appType,
			sourceType,
			clusterName,
			status,
			message)
	}
	w.Flush()

	return nil
}

func extractMessage(item *unstructured.Unstructured) (string, error) {
	status, found, err := unstructured.NestedMap(item.Object, "status")
	if !found || err != nil {
		return "", fmt.Errorf("status field not found or error occurred: %v", err)
	}

	resources, found, err := unstructured.NestedSlice(status, "resources")
	if !found || err != nil {
		return "", fmt.Errorf("resources field not found or error occurred: %v", err)
	}

	for _, resource := range resources {
		resourceMap, ok := resource.(map[string]interface{})
		if !ok {
			continue
		}

		health, found, err := unstructured.NestedMap(resourceMap, "health")
		if found && err == nil {
			message, found, err := unstructured.NestedString(health, "message")
			if found && err == nil {
				return message, nil
			}
		}
	}

	return "", fmt.Errorf("message not found")
}

func extractStatus(item *unstructured.Unstructured) (string, error) {
	status, found, err := unstructured.NestedMap(item.Object, "status")
	if !found || err != nil {
		return "", fmt.Errorf("status field not found or error occurred: %v", err)
	}

	healthStatus, found, err := unstructured.NestedString(status, "health", "status")
	if !found || err != nil {
		return "", fmt.Errorf("health.status field not found or error occurred: %v", err)
	}

	return healthStatus, nil
}
