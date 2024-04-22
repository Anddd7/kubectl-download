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

	errNoContext  = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
	errNoResource = fmt.Errorf("no such resource found")
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
	restConfig *rest.Config
	timestamp  int64
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

			list, err := o.getValidResourceNames()
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
	configLoader := o.configFlags.ToRawKubeConfigLoader()
	restConfig, err := configLoader.ClientConfig()
	if err != nil {
		return err
	}
	rawConfig, err := configLoader.RawConfig()
	if err != nil {
		return err
	}

	context := getFlagOrDefault(cmd, "context", rawConfig.CurrentContext)
	targetContext, exists := rawConfig.Contexts[context]
	if !exists {
		return errNoContext
	}

	// override flags
	o.args = args
	o.namespace = getFlagOrDefault(cmd, "namespace", targetContext.Namespace)
	o.context = context
	o.user = getFlagOrDefault(cmd, "user", targetContext.AuthInfo)
	o.restConfig = restConfig
	o.timestamp = metav1.Now().Unix()

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
	resource, err := o.getAPIResource(kind)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{
		Group:    resource.Group,
		Version:  resource.Version,
		Resource: resource.Name,
	}

	dynamicClient, err := dynamic.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	var unstructured *unstructured.UnstructuredList
	if resource.Namespaced {
		unstructured, err = dynamicClient.Resource(gvr).Namespace(o.namespace).List(context.TODO(), metav1.ListOptions{})
	} else {
		unstructured, err = dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
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
		filename := o.getFilename(gvr, name)
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

func (o *CommandOptions) filterServerSideFields(_ *unstructured.Unstructured) {
	if o.clientFormat {
		slog.Debug("TODO: filter server side fields")
	}
}

func (o *CommandOptions) downloadTargetResource(kind string, name string) error {
	resource, err := o.getAPIResource(kind)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{
		Group:    resource.Group,
		Version:  resource.Version,
		Resource: resource.Name,
	}

	dynamicClient, err := dynamic.NewForConfig(o.restConfig)
	if err != nil {
		return err
	}

	var unstructured *unstructured.Unstructured
	if resource.Namespaced {
		unstructured, err = dynamicClient.Resource(gvr).Namespace(o.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		unstructured, err = dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	}
	if err != nil {
		return err
	}

	o.filterServerSideFields(unstructured)
	content, err := yaml.Marshal(unstructured.Object)
	if err != nil {
		return err
	}

	filename := o.getFilename(gvr, name)
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("downloaded: %s\n", filename)

	return nil
}

func (o *CommandOptions) getValidResourceNames() ([]string, error) {
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

func (o *CommandOptions) getAPIResource(kind string) (*metav1.APIResource, error) {
	inputGVR, inputResource := schema.ParseResourceArg(kind)
	if inputGVR == nil {
		withoutVersion := inputResource.WithVersion("")
		inputGVR = &withoutVersion
	}
	slog.Debug("parsed input resource",
		"group-v", inputGVR.Group, "version-v", inputGVR.Version,
		"group", inputResource.Group, "resource", inputResource.Resource,
	)

	dc, err := o.configFlags.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	resourceLists, err := dc.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for i := range resourceList.APIResources {
			resource := &resourceList.APIResources[i]
			resource.Group = gv.Group
			resource.Version = gv.Version

			if isMatchedResource(inputGVR, inputResource, resource) {
				slog.Debug("got matched resource",
					"group", resource.Group, "version", resource.Version, "resource", resource.Name, "namespaced", resource.Namespaced, "kind", resource.Kind,
				)
				return resource, nil
			}
		}
	}

	return nil, errNoResource
}

func isMatchedResource(gvr *schema.GroupVersionResource, res schema.GroupResource, resource *metav1.APIResource) bool {
	// assume gvr is valid
	isGroupMatched := gvr.Group == "" || gvr.Group == resource.Group
	isVersionMatched := gvr.Version == "" || gvr.Version == resource.Version
	isResourceMatched := gvr.Resource == resource.Name || gvr.Resource == resource.SingularName || gvr.Resource == resource.Kind ||
		// tips:: to match resource without singular(i don't know why), e.g. ingress
		gvr.Resource == strings.ToLower(resource.Kind)

	for _, shortName := range resource.ShortNames {
		if gvr.Resource == shortName {
			isResourceMatched = true
		}
	}

	if isGroupMatched && isVersionMatched && isResourceMatched {
		return true
	}

	// slog.Debug("[GVR] resource not matched - "+resource.Name,
	// 	"isGroupMatched", isGroupMatched,
	// 	"isVersionMatched", isVersionMatched,
	// 	"isResourceMatched", isResourceMatched,
	// )

	// assume gvr is invalid (version is empty)
	isGroupMatched = res.Group == "" || res.Group == resource.Group

	if isGroupMatched && isResourceMatched {
		return true
	}

	// slog.Debug("[GR] resource not matched - "+resource.Name,
	// 	"isGroupMatched", isGroupMatched,
	// 	"isResourceMatched", isResourceMatched,
	// )

	return false
}
