package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
)

var (
	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

func Execute() {
	if err := NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

type GenericOptions struct {
	configFlags *genericclioptions.ConfigFlags
	ioStreams   genericiooptions.IOStreams

	args []string

	restConfig *rest.Config
	rawConfig  api.Config

	namespace string
	context   string
	user      string
}

func NewCommand() *cobra.Command {
	o := &GenericOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		ioStreams:   genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "kubectl-download [kind] [name]",
		Short: "download resource into a file",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			slog.Info("download-cmd: complete")
			slog.Info("genericOptions: ", "namespace", o.namespace, "context", o.context, "user", o.user)
			slog.Info(fmt.Sprintf("args %v", o.args))

			if err := o.Validate(); err != nil {
				return err
			}
			slog.Info("download-cmd: validate")

			if err := o.Run(); err != nil {
				return err
			}
			slog.Info("download-cmd: run")

			return nil
		},
	}

	o.configFlags.AddFlags(cmd.Flags())
	return cmd
}

func (o *GenericOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error

	o.args = args

	configLoader := o.configFlags.ToRawKubeConfigLoader()
	o.restConfig, err = configLoader.ClientConfig()
	if err != nil {
		return err
	}
	o.rawConfig, err = configLoader.RawConfig()
	if err != nil {
		return err
	}

	o.context = getFlagOrDefault(cmd, "context", o.rawConfig.CurrentContext)
	currentContext, exists := o.rawConfig.Contexts[o.context]
	if !exists {
		return errNoContext
	}

	o.namespace = getFlagOrDefault(cmd, "namespace", currentContext.Namespace)
	o.user = getFlagOrDefault(cmd, "user", currentContext.AuthInfo)

	return nil
}

func getFlagOrDefault(cmd *cobra.Command, name string, defaultValue string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil || len(value) == 0 {
		return defaultValue
	}
	return value
}

func (o *GenericOptions) Validate() error {
	if len(o.context) == 0 {
		return errNoContext
	}
	if len(o.args) == 0 {
		return fmt.Errorf("kind is required")
	}
	if len(o.args) > 2 {
		return fmt.Errorf("too many arguments, only kind and name are allowed")
	}

	return nil
}

func (o *GenericOptions) Run() error {
	if len(o.args) == 1 {
		return o.downloadAllResources(o.args[0])
	}

	if len(o.args) == 2 {
		return o.downloadTargetResource(o.args[0], o.args[1])
	}

	return nil
}

func (o *GenericOptions) downloadAllResources(kind string) error {
	clientset, err := kubernetes.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	resources, err := clientset.AppsV1().Deployments(o.namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, resource := range resources.Items {
		fmt.Println(resource.Name)
	}

	return nil
}

func (o *GenericOptions) downloadTargetResource(kind string, name string) error {
	dynamicClient, err := dynamic.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: kind}

	unstructuredList, err := dynamicClient.Resource(gvr).Namespace(o.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, resource := range unstructuredList.Items {
		fmt.Println(resource.GetName())
	}

	return nil
}
