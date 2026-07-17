package cmd

import "k8s.io/client-go/tools/clientcmd"

// resolveNamespace returns flag when set; otherwise the kubeconfig
// current-context namespace, falling back to "default" when no config or
// context namespace is reachable.
func resolveNamespace(flag string) string {
	if flag != "" {
		return flag
	}

	ns, _, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{},
	).Namespace()

	if err != nil || ns == "" {
		return "default"
	}
	return ns
}
