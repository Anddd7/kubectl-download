package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/util"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	shortText = "Output and download any kubernetes resources in to named files."

	helperText = `
# download all pods
kubectl donwload pod

# download ingress in a specific namespace
kubectl download ingress my-ingress -n my-namespace

# download pod in dist folder with prefix and suffix
kubectl download pod my-pod --prefix my-prefix --suffix my-suffix -o dist
	`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

func Execute() {
	if err := NewCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

type CommandOptions struct {
	// cli-runtime
	configFlags *genericclioptions.ConfigFlags
	ioStreams   genericiooptions.IOStreams

	// input args
	args []string

	debug bool

	// kubectl flags
	namespace string
	context   string
	user      string
	// flags
	output          string
	clientFormat    bool
	prefix          string
	suffix          string
	suffixTimestamp bool

	// internal state
	utilFactory util.Factory
	restConfig  *rest.Config
	rawConfig   api.Config
	timestamp   int64
}

func NewCommand() *cobra.Command {
	o := &CommandOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		ioStreams:   genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:     "kubectl-download [kind] [name]",
		Short:   shortText,
		Example: helperText,
		Args:    cobra.MaximumNArgs(2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			list, err := o.getValidResources()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return list, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(c *cobra.Command, args []string) error {
			if o.debug {
				opts := slog.HandlerOptions{Level: slog.LevelDebug}
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &opts)))
			}

			if err := o.Complete(c, args); err != nil {
				return err
			}
			slog.Debug("genericOptions: ", "namespace", o.namespace, "context", o.context, "user", o.user)
			slog.Debug(fmt.Sprintf("args %v", o.args))

			if err := o.Validate(); err != nil {
				return err
			}

			if err := o.Run(); err != nil {
				fmt.Printf("failed: %s\n", err)
				return nil
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&o.debug, "debug", false, "enable debug logging")

	cmd.Flags().StringVarP(&o.output, "output", "o", ".", "output path")
	cmd.Flags().BoolVar(&o.clientFormat, "client", false, "only output client applicable fields")
	cmd.Flags().StringVar(&o.prefix, "prefix", "", "add prefix to the filename")
	cmd.Flags().StringVar(&o.suffix, "suffix", "", "add suffix to the filename")
	cmd.Flags().BoolVar(&o.suffixTimestamp, "suffix-timestamp", false, "add timestamp as suffix to the filename")

	// manually add kubectl flags
	if o.configFlags.Context != nil {
		cmd.Flags().StringVar(o.configFlags.Context, "context", *o.configFlags.Context, "The name of the kubeconfig context to use")
	}
	if o.configFlags.Username != nil {
		cmd.Flags().StringVar(o.configFlags.Namespace, "namespace", *o.configFlags.Namespace, "If present, the namespace scope for this CLI request")
	}
	if o.configFlags.Username != nil {
		cmd.Flags().StringVar(o.configFlags.Username, "user", *o.configFlags.Username, "The name of the kubeconfig user to use")
	}

	return cmd
}

func (o *CommandOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error

	o.args = args

	// build internal state
	o.utilFactory = util.NewFactory(o.configFlags)

	configLoader := o.configFlags.ToRawKubeConfigLoader()
	o.restConfig, err = configLoader.ClientConfig()
	if err != nil {
		return err
	}
	o.rawConfig, err = configLoader.RawConfig()
	if err != nil {
		return err
	}

	o.timestamp = metav1.Now().Unix()

	// set kubectl flags
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

func (o *CommandOptions) Validate() error {
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

func (o *CommandOptions) Run() error {
	if len(o.args) == 1 {
		return o.downloadAllResources(o.args[0])
	}

	if len(o.args) == 2 {
		return o.downloadTargetResource(o.args[0], o.args[1])
	}

	return nil
}

func (o *CommandOptions) downloadAllResources(kind string) error {
	mapping, err := o.parseResourceRestMapping(kind)
	if err != nil {
		return err
	}

	slog.Debug("found resource", "scope", mapping.Scope, "group", mapping.GroupVersionKind.Group, "version", mapping.GroupVersionKind.Version, "resource", mapping.GroupVersionKind.Kind)

	dynamicClient, err := dynamic.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	var unstructured *unstructured.UnstructuredList
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		unstructured, err = dynamicClient.Resource(mapping.Resource).Namespace(o.namespace).List(context.TODO(), metav1.ListOptions{})
	} else {
		unstructured, err = dynamicClient.Resource(mapping.Resource).List(context.TODO(), metav1.ListOptions{})
	}
	if err != nil {
		return err
	}

	for _, item := range unstructured.Items {
		o.filterServerSideFields(&item)
		content, err := yaml.Marshal(item.Object)
		if err != nil {
			return err
		}

		name := item.Object["metadata"].(map[string]interface{})["name"].(string)
		filename := o.getFilename(mapping.Resource, name)
		err = os.WriteFile(filename, content, 0644)
		if err != nil {
			return err
		}

		fmt.Printf("downloaded: %s\n", filename)
	}

	return nil
}

func (o *CommandOptions) getFilename(gvr schema.GroupVersionResource, name string) string {
	var filenames []string
	if o.prefix != "" {
		filenames = append(filenames, o.prefix)
	}
	filenames = append(filenames, gvr.Resource, name)
	if o.suffix != "" {
		filenames = append(filenames, o.suffix)
	}
	if o.suffixTimestamp {
		filenames = append(filenames, fmt.Sprintf("%d", o.timestamp))
	}

	filename := strings.Join(filenames, "_") + ".yaml"

	if o.output != "." {
		if _, err := os.Stat(o.output); os.IsNotExist(err) {
			err := os.Mkdir(o.output, 0755)
			if err != nil {
				slog.Debug("failed to create output directory", "error", err)
			}
		}
		filename = filepath.Join(o.output, filename)
	}
	return filename
}

func (o *CommandOptions) filterServerSideFields(unstructured *unstructured.Unstructured) {
	if o.clientFormat {
		slog.Debug("TODO: filter server side fields")
	}
}

func (o *CommandOptions) downloadTargetResource(kind string, name string) error {
	mapping, err := o.parseResourceRestMapping(kind)
	if err != nil {
		return err
	}

	slog.Debug("found resource", "scope", mapping.Scope, "group", mapping.GroupVersionKind.Group, "version", mapping.GroupVersionKind.Version, "resource", mapping.GroupVersionKind.Kind)

	dynamicClient, err := dynamic.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	var unstructured *unstructured.Unstructured
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		unstructured, err = dynamicClient.Resource(mapping.Resource).Namespace(o.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		unstructured, err = dynamicClient.Resource(mapping.Resource).Get(context.TODO(), name, metav1.GetOptions{})
	}
	if err != nil {
		return err
	}

	o.filterServerSideFields(unstructured)
	content, err := yaml.Marshal(unstructured.Object)
	if err != nil {
		return err
	}

	filename := o.getFilename(mapping.Resource, name)
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("downloaded: %s\n", filename)

	return nil
}

func (o *CommandOptions) getValidResources() ([]string, error) {
	dc, err := o.configFlags.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	resourceLists, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	// Create a set of all resource names
	resourceNames := make(map[string]struct{})
	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			resourceNames[resource.Name] = struct{}{}
		}
	}

	// Convert the set to a slice
	var list []string
	for resourceName := range resourceNames {
		list = append(list, resourceName)
	}
	return list, nil
}

func (o *CommandOptions) parseResourceRestMapping(kind string) (*meta.RESTMapping, error) {
	restMapper, err := o.utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	gvr, resource := schema.ParseResourceArg(kind)
	if gvr == nil {
		slog.Debug("version is empty")

		withoutVersion := resource.WithVersion("")
		gvr = &withoutVersion
	}

	slog.Debug("parsed resource", "group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource)

	gvk, err := restMapper.KindFor(*gvr)
	if err != nil {
		_, kind := schema.ParseKindArg(kind)

		slog.Debug("group/version invalid, parse kind", "group", kind.Group, "kind", kind.Kind)

		return restMapper.RESTMapping(kind, "")
	}

	slog.Debug("found fully specific kind for resource", "group", gvk.Group, "version", gvk.Version, "kind", gvk.Kind)

	return restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
}
