package pkg

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetAPIResources() ([]string, error) {
	// load default kubeconfig
	restConfig, err := getDefaultRestConfig()
	if err != nil {
		return nil, err
	}

	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// Get the list of all API resources
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

func getDefaultRestConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	return clientConfig.ClientConfig()
}
