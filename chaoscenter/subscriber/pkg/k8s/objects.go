package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"subscriber/pkg/types"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	InfraNamespace      = os.Getenv("INFRA_NAMESPACE")
	TargetAppNamespaces = os.Getenv("TARGET_APP_NAMESPACES")
)

// candidateNamespaces returns the deduplicated list of namespace names this
// infra is allowed to resolve: its own namespace plus whatever
// TARGET_APP_NAMESPACES lists. Kept in lockstep with the `get`+resourceNames
// rule ManifestParser renders into infra-cluster-role (3a_agents_rbac.yaml)
// from the same app-charts catalog — see GetKubernetesNamespaces below for
// why this can't just be a cluster-wide List().
func candidateNamespaces() []string {
	seen := map[string]bool{InfraNamespace: true}
	names := []string{InfraNamespace}
	for _, raw := range strings.Split(TargetAppNamespaces, ",") {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// GetKubernetesNamespaces is used to get the list of Kubernetes Namespaces.
//
// This deliberately does NOT branch on InfraScope the way it historically
// did (namespace scope => only ever report InfraNamespace itself). ACE
// connects one infra per orchestration namespace (e.g. "itbench") but that
// infra still needs to inject chaos into separate target-application
// namespaces (sock-shop, book-info, otel-demo, ...) it doesn't own — so the
// UI's namespace picker needs visibility into those too, regardless of how
// this infra's engine/workflow watching is scoped elsewhere (see InfraScope
// usage in pkg/events for that unrelated concern).
//
// This service account is intentionally NOT granted list/watch on
// namespaces: Kubernetes RBAC can't scope those verbs to a resourceNames
// subset, so a List() here would require cluster-wide visibility into every
// namespace on the cluster, including ones unrelated to chaos experiments.
// Instead it holds `get` on a known allowlist (see infra-cluster-role /
// infra-role in the manifests) — resolve each candidate individually and
// skip whichever don't exist yet (not-yet-onboarded apps) or aren't
// reachable under this infra's RBAC (a genuinely namespace-scoped infra that
// was never granted the cluster-scope get either).
func (k8s *k8sSubscriber) GetKubernetesNamespaces(request types.KubeNamespaceRequest) ([]*types.KubeNamespace, error) {

	var namespaceData []*types.KubeNamespace

	conf, err := k8s.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, err
	}

	for _, name := range candidateNamespaces() {
		ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if k8s_errors.IsNotFound(err) || k8s_errors.IsForbidden(err) {
				continue
			}
			return nil, err
		}
		namespaceData = append(namespaceData, &types.KubeNamespace{Name: ns.GetName()})
	}

	if len(namespaceData) == 0 {
		return nil, errors.New("No namespace available")
	}
	//TODO Maybe add marshal/unmarshal here
	return namespaceData, nil
}

// GetKubernetesObjects is used to get the Kubernetes Object details according to the request type
func (k8s *k8sSubscriber) GetKubernetesObjects(request types.KubeObjRequest) (*types.KubeObject, error) {
	resourceType := schema.GroupVersionResource{
		Group:    request.KubeGVRRequest.Group,
		Version:  request.KubeGVRRequest.Version,
		Resource: request.KubeGVRRequest.Resource,
	}
	_, dynamicClient, err := k8s.GetDynamicAndDiscoveryClient()
	if err != nil {
		return nil, err
	}

	dataList, err := k8s.GetObjectDataByNamespace(request.Namespace, dynamicClient, resourceType)
	if err != nil {
		return nil, err
	}
	KubeObj := &types.KubeObject{
		Namespace: InfraNamespace,
		Data:      dataList,
	}

	kubeData, _ := json.Marshal(KubeObj)
	var kubeObjects *types.KubeObject
	err = json.Unmarshal(kubeData, &kubeObjects)
	if err != nil {
		return nil, err
	}
	return kubeObjects, nil
}

// GetObjectDataByNamespace uses dynamic client to fetch Kubernetes Objects data.
func (k8s *k8sSubscriber) GetObjectDataByNamespace(namespace string, dynamicClient dynamic.Interface, resourceType schema.GroupVersionResource) ([]types.ObjectData, error) {
	list, err := dynamicClient.Resource(resourceType).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	var kubeObjects []types.ObjectData
	if err != nil {
		return kubeObjects, nil
	}
	for _, list := range list.Items {
		listInfo := types.ObjectData{
			Name:                    list.GetName(),
			UID:                     list.GetUID(),
			Namespace:               list.GetNamespace(),
			APIVersion:              list.GetAPIVersion(),
			CreationTimestamp:       list.GetCreationTimestamp(),
			TerminationGracePeriods: list.GetDeletionGracePeriodSeconds(),
			Labels:                  k8s.updateLabels(list.GetLabels()),
		}
		kubeObjects = append(kubeObjects, listInfo)
	}
	return kubeObjects, nil
}

func (k8s *k8sSubscriber) updateLabels(labels map[string]string) []string {
	var updatedLabels []string

	for k, v := range labels {
		updatedLabels = append(updatedLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return updatedLabels
}

func (k8s *k8sSubscriber) GenerateKubeNamespace(cid string, accessKey, version string, kubenamespacerequest types.KubeNamespaceRequest) ([]byte, error) {
	infraID := `{infraID: \"` + cid + `\", version: \"` + version + `\", accessKey: \"` + accessKey + `\"}`
	kubeObj, err := k8s.GetKubernetesNamespaces(kubenamespacerequest)
	if err != nil {
		return nil, err
	}
	processed, err := k8s.gqlSubscriberServer.MarshalGQLData(kubeObj)
	if err != nil {
		return nil, err
	}
	mutation := `{ infraID: ` + infraID + `, requestID:\"` + kubenamespacerequest.RequestID + `\", kubeNamespace:\"` + processed[1:len(processed)-1] + `\"}`

	var payload = []byte(`{"query":"mutation { kubeNamespace(request:` + mutation + ` )}"}`)
	return payload, nil
}

func (k8s *k8sSubscriber) GenerateKubeObject(cid string, accessKey, version string, kubeobjectrequest types.KubeObjRequest) ([]byte, error) {
	infraID := `{infraID: \"` + cid + `\", version: \"` + version + `\", accessKey: \"` + accessKey + `\"}`
	kubeObj, err := k8s.GetKubernetesObjects(kubeobjectrequest)
	if err != nil {
		return nil, err
	}
	processed, err := k8s.gqlSubscriberServer.MarshalGQLData(kubeObj)
	if err != nil {
		return nil, err
	}
	mutation := `{ infraID: ` + infraID + `, requestID:\"` + kubeobjectrequest.RequestID + `\", kubeObj:\"` + processed[1:len(processed)-1] + `\"}`

	var payload = []byte(`{"query":"mutation { kubeObj(request:` + mutation + ` )}"}`)
	return payload, nil
}

// SendKubeNamespace generates graphql mutation to send kubernetes namespaces data to graphql server
func (k8s *k8sSubscriber) SendKubeNamespaces(infraData map[string]string, kubenamespacerequest types.KubeNamespaceRequest) error {
	// generate graphql payload
	payload, err := k8s.GenerateKubeNamespace(infraData["INFRA_ID"], infraData["ACCESS_KEY"], infraData["VERSION"], kubenamespacerequest)
	if err != nil {
		logrus.WithError(err).Print("Error while getting KubeObject Data")
		return err
	}

	body, err := k8s.gqlSubscriberServer.SendRequest(infraData["SERVER_ADDR"], payload)
	if err != nil {
		logrus.Print(err.Error())
		return err
	}

	logrus.Println("Response", body)
	return nil
}

// SendKubeObjects generates graphql mutation to send kubernetes objects data to graphql server
func (k8s *k8sSubscriber) SendKubeObjects(infraData map[string]string, kubeobjectrequest types.KubeObjRequest) error {
	// generate graphql payload
	payload, err := k8s.GenerateKubeObject(infraData["INFRA_ID"], infraData["ACCESS_KEY"], infraData["VERSION"], kubeobjectrequest)
	if err != nil {
		logrus.WithError(err).Print("Error while getting KubeObject Data")
		return err
	}

	body, err := k8s.gqlSubscriberServer.SendRequest(infraData["SERVER_ADDR"], payload)
	if err != nil {
		logrus.Print(err.Error())
		return err
	}

	logrus.Println("Response", body)
	return nil
}
