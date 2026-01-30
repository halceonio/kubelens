package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/halceonio/kubelens/backend/internal/auth"
)

type namespaceResponse struct {
	Name string `json:"name"`
}

type resourceUsage struct {
	CPUUsage   string `json:"cpuUsage"`
	CPURequest string `json:"cpuRequest"`
	CPULimit   string `json:"cpuLimit"`
	MemUsage   string `json:"memUsage"`
	MemRequest string `json:"memRequest"`
	MemLimit   string `json:"memLimit"`
}

type containerResponse struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
}

type volumeMountResponse struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly"`
}

type podResponse struct {
	Name        string                `json:"name"`
	Namespace   string                `json:"namespace"`
	Status      string                `json:"status"`
	Restarts    int32                 `json:"restarts"`
	Age         string                `json:"age"`
	Labels      map[string]string     `json:"labels"`
	Annotations map[string]string     `json:"annotations"`
	Env         map[string]string     `json:"env"`
	Containers  []containerResponse   `json:"containers"`
	Volumes     []volumeMountResponse `json:"volumes"`
	Secrets     []string              `json:"secrets"`
	ConfigMaps  []string              `json:"configMaps"`
	Resources   resourceUsage         `json:"resources"`
	OwnerApp    string                `json:"ownerApp,omitempty"`
}

type appResponse struct {
	Name          string                `json:"name"`
	Namespace     string                `json:"namespace"`
	Type          string                `json:"type"`
	Replicas      int32                 `json:"replicas"`
	ReadyReplicas int32                 `json:"readyReplicas"`
	PodNames      []string              `json:"podNames"`
	Labels        map[string]string     `json:"labels"`
	Annotations   map[string]string     `json:"annotations"`
	Env           map[string]string     `json:"env"`
	Resources     resourceUsage         `json:"resources"`
	Volumes       []volumeMountResponse `json:"volumes"`
	Secrets       []string              `json:"secrets"`
	ConfigMaps    []string              `json:"configMaps"`
	Image         string                `json:"image,omitempty"`
}

const (
	cnpgClusterListPathFmt = "/apis/postgresql.cnpg.io/v1/namespaces/%s/clusters"
	cnpgClusterGetPathFmt  = "/apis/postgresql.cnpg.io/v1/namespaces/%s/clusters/%s"
	dragonflyListPathFmt   = "/apis/dragonflydb.io/v1alpha1/namespaces/%s/dragonflies"
	dragonflyGetPathFmt    = "/apis/dragonflydb.io/v1alpha1/namespaces/%s/dragonflies/%s"
	cnpgClusterLabelKey    = "cnpg.io/cluster"
	dragonflyAppLabelKey   = "app"
	dragonflyOwnerKind     = "Dragonfly"
)

type cnpgClusterList struct {
	Items []cnpgCluster `json:"items"`
}

type cnpgCluster struct {
	Metadata metav1.ObjectMeta `json:"metadata"`
	Spec     cnpgClusterSpec   `json:"spec"`
	Status   cnpgClusterStatus `json:"status"`
}

type cnpgClusterSpec struct {
	Instances *int32                      `json:"instances"`
	ImageName string                      `json:"imageName"`
	Resources corev1.ResourceRequirements `json:"resources"`
}

type cnpgClusterStatus struct {
	ReadyInstances int32    `json:"readyInstances"`
	InstanceNames  []string `json:"instanceNames"`
	Image          string   `json:"image"`
}

type dragonflyList struct {
	Items []dragonflyResource `json:"items"`
}

type dragonflyResource struct {
	Metadata metav1.ObjectMeta `json:"metadata"`
	Spec     dragonflySpec     `json:"spec"`
	Status   dragonflyStatus   `json:"status"`
}

type dragonflySpec struct {
	Image     string                      `json:"image"`
	Replicas  *int32                      `json:"replicas"`
	Env       []corev1.EnvVar             `json:"env"`
	Resources corev1.ResourceRequirements `json:"resources"`
}

type dragonflyStatus struct {
	Phase string `json:"phase"`
}

func (h *KubeHandler) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := make([]namespaceResponse, 0, len(h.cfg.Kubernetes.AllowedNamespaces))
	for _, ns := range h.cfg.Kubernetes.AllowedNamespaces {
		resp = append(resp, namespaceResponse{Name: ns})
	}
	writeJSON(w, resp)
}

func (h *KubeHandler) handlePods(w http.ResponseWriter, r *http.Request, namespace string, parts []string) {
	if len(parts) == 0 {
		h.handlePodsList(w, r, namespace)
		return
	}
	name := parts[0]
	if len(parts) == 1 {
		h.handlePodGet(w, r, namespace, name)
		return
	}
	sub := parts[1]
	switch sub {
	case "logs":
		h.streamPodLogs(w, r, namespace, name)
	case "details":
		h.handlePodDetails(w, r, namespace, name)
	case "metrics":
		h.handlePodMetrics(w, r, namespace, name)
	default:
		http.NotFound(w, r)
	}
}

func (h *KubeHandler) handleApps(w http.ResponseWriter, r *http.Request, namespace string, parts []string) {
	if len(parts) == 0 {
		h.handleAppsList(w, r, namespace)
		return
	}
	name := parts[0]
	if len(parts) == 1 {
		h.handleAppGet(w, r, namespace, name)
		return
	}
	sub := parts[1]
	switch sub {
	case "logs":
		h.streamAppLogs(w, r, namespace, name)
	default:
		http.NotFound(w, r)
	}
}

func (h *KubeHandler) handlePodsList(w http.ResponseWriter, r *http.Request, namespace string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	pods, err := h.client.CoreV1().Pods(namespace).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	resp := make([]podResponse, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if !h.allowPod(&pod) {
			continue
		}
		resp = append(resp, h.mapPod(&pod, false, nil))
	}
	writeJSON(w, resp)
}

func (h *KubeHandler) handlePodGet(w http.ResponseWriter, r *http.Request, namespace, name string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	pod, err := h.client.CoreV1().Pods(namespace).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if !h.allowPod(pod) {
		writeError(w, http.StatusForbidden, "pod not allowed")
		return
	}
	writeJSON(w, h.mapPod(pod, false, nil))
}

func (h *KubeHandler) handlePodDetails(w http.ResponseWriter, r *http.Request, namespace, name string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	pod, err := h.client.CoreV1().Pods(namespace).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if !h.allowPod(pod) {
		writeError(w, http.StatusForbidden, "pod not allowed")
		return
	}

	var user *auth.User
	if u, ok := auth.UserFromContext(r.Context()); ok {
		user = u
	}
	writeJSON(w, h.mapPod(pod, true, user))
}

func (h *KubeHandler) handleAppsList(w http.ResponseWriter, r *http.Request, namespace string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	resp := []appResponse{}

	deployments, err := h.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, dep := range deployments.Items {
		if !h.allowApp(dep.Name, dep.Labels) {
			continue
		}
		resp = append(resp, h.mapDeployment(ctx, &dep))
	}

	statefulSets, err := h.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, sts := range statefulSets.Items {
		if hasOwnerKind(sts.OwnerReferences, dragonflyOwnerKind) {
			continue
		}
		if !h.allowApp(sts.Name, sts.Labels) {
			continue
		}
		resp = append(resp, h.mapStatefulSet(ctx, &sts))
	}

	cnpgClusters, err := h.listCnpgClusters(ctx, namespace)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, cluster := range cnpgClusters {
		if !h.allowApp(cluster.Metadata.Name, cluster.Metadata.Labels) {
			continue
		}
		resp = append(resp, h.mapCnpgCluster(ctx, &cluster))
	}

	dragonflies, err := h.listDragonflies(ctx, namespace)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, dragonfly := range dragonflies {
		if !h.allowApp(dragonfly.Metadata.Name, dragonfly.Metadata.Labels) {
			continue
		}
		resp = append(resp, h.mapDragonfly(ctx, &dragonfly))
	}

	writeJSON(w, resp)
}

func (h *KubeHandler) handleAppGet(w http.ResponseWriter, r *http.Request, namespace, name string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	dep, err := h.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if !h.allowApp(dep.Name, dep.Labels) {
			writeError(w, http.StatusForbidden, "app not allowed")
			return
		}
		writeJSON(w, h.mapDeployment(ctx, dep))
		return
	}
	sts, err := h.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if hasOwnerKind(sts.OwnerReferences, dragonflyOwnerKind) {
			writeError(w, http.StatusNotFound, "app not found")
			return
		}
		if !h.allowApp(sts.Name, sts.Labels) {
			writeError(w, http.StatusForbidden, "app not allowed")
			return
		}
		writeJSON(w, h.mapStatefulSet(ctx, sts))
		return
	}
	cluster, err := h.getCnpgCluster(ctx, namespace, name)
	if err == nil {
		if !h.allowApp(cluster.Metadata.Name, cluster.Metadata.Labels) {
			writeError(w, http.StatusForbidden, "app not allowed")
			return
		}
		writeJSON(w, h.mapCnpgCluster(ctx, cluster))
		return
	}
	if err != nil && !apierrors.IsNotFound(err) {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	dragonfly, err := h.getDragonfly(ctx, namespace, name)
	if err == nil {
		if !h.allowApp(dragonfly.Metadata.Name, dragonfly.Metadata.Labels) {
			writeError(w, http.StatusForbidden, "app not allowed")
			return
		}
		writeJSON(w, h.mapDragonfly(ctx, dragonfly))
		return
	}
	if err != nil && !apierrors.IsNotFound(err) {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeError(w, http.StatusNotFound, "app not found")
}

func (h *KubeHandler) handlePodMetrics(w http.ResponseWriter, r *http.Request, namespace, name string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	pod, err := h.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if !h.allowPod(pod) {
		writeError(w, http.StatusForbidden, "pod not allowed")
		return
	}

	usage, err := h.fetchPodMetrics(ctx, namespace, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	requests, limits := sumResourceRequests(pod.Spec.Containers)
	usage.CPURequest = requests.cpu.String()
	usage.MemRequest = requests.mem.String()
	usage.CPULimit = limits.cpu.String()
	usage.MemLimit = limits.mem.String()
	writeJSON(w, usage)
}

func (h *KubeHandler) allowPod(pod *corev1.Pod) bool {
	if !h.podInclude.MatchString(pod.Name) {
		return false
	}
	if matchesExcluded(pod.Labels, h.podExclude) {
		return false
	}
	return true
}

func (h *KubeHandler) allowApp(name string, labels map[string]string) bool {
	if !h.appInclude.MatchString(name) {
		return false
	}
	if matchesExcluded(labels, h.appExclude) {
		return false
	}
	return true
}

func (h *KubeHandler) mapPod(pod *corev1.Pod, includeDetails bool, user *auth.User) podResponse {
	restarts := int32(0)
	containers := make([]containerResponse, 0, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
		containers = append(containers, containerResponse{
			Name:         status.Name,
			Image:        status.Image,
			Ready:        status.Ready,
			RestartCount: status.RestartCount,
		})
	}

	volumes := extractVolumeMounts(pod.Spec.Containers)
	secrets, configMaps := extractSecretsConfigMaps(pod.Spec.Containers, pod.Spec.Volumes)
	requests, limits := sumResourceRequests(pod.Spec.Containers)

	usage := resourceUsage{
		CPUUsage:   "0",
		MemUsage:   "0",
		CPURequest: requests.cpu.String(),
		MemRequest: requests.mem.String(),
		CPULimit:   limits.cpu.String(),
		MemLimit:   limits.mem.String(),
	}

	env := map[string]string{}
	if len(pod.Spec.Containers) > 0 {
		env = extractEnv(pod.Namespace, pod.Spec.Containers[0].Env, user, h.client)
	}

	return podResponse{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		Status:      string(pod.Status.Phase),
		Restarts:    restarts,
		Age:         formatAge(pod.CreationTimestamp.Time),
		Labels:      pod.Labels,
		Annotations: pod.Annotations,
		Env:         env,
		Containers:  containers,
		Volumes:     volumes,
		Secrets:     secrets,
		ConfigMaps:  configMaps,
		Resources:   usage,
		OwnerApp:    ownerRefName(pod.OwnerReferences),
	}
}

func (h *KubeHandler) mapDeployment(ctx context.Context, dep *appsv1.Deployment) appResponse {
	pods := h.listPodsForSelector(ctx, dep.Namespace, dep.Spec.Selector)
	requests, limits := sumResourceRequests(dep.Spec.Template.Spec.Containers)
	volumes := extractVolumeMounts(dep.Spec.Template.Spec.Containers)
	secrets, configMaps := extractSecretsConfigMaps(dep.Spec.Template.Spec.Containers, dep.Spec.Template.Spec.Volumes)
	image := ""
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		image = dep.Spec.Template.Spec.Containers[0].Image
	}

	return appResponse{
		Name:          dep.Name,
		Namespace:     dep.Namespace,
		Type:          "Deployment",
		Replicas:      derefInt32(dep.Spec.Replicas),
		ReadyReplicas: dep.Status.ReadyReplicas,
		PodNames:      pods,
		Labels:        dep.Labels,
		Annotations:   dep.Annotations,
		Env:           extractEnv(dep.Namespace, firstEnv(dep.Spec.Template.Spec.Containers), nil, nil),
		Resources: resourceUsage{
			CPUUsage:   "0",
			MemUsage:   "0",
			CPURequest: requests.cpu.String(),
			MemRequest: requests.mem.String(),
			CPULimit:   limits.cpu.String(),
			MemLimit:   limits.mem.String(),
		},
		Volumes:    volumes,
		Secrets:    secrets,
		ConfigMaps: configMaps,
		Image:      image,
	}
}

func (h *KubeHandler) mapStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) appResponse {
	pods := h.listPodsForSelector(ctx, sts.Namespace, sts.Spec.Selector)
	requests, limits := sumResourceRequests(sts.Spec.Template.Spec.Containers)
	volumes := extractVolumeMounts(sts.Spec.Template.Spec.Containers)
	secrets, configMaps := extractSecretsConfigMaps(sts.Spec.Template.Spec.Containers, sts.Spec.Template.Spec.Volumes)
	image := ""
	if len(sts.Spec.Template.Spec.Containers) > 0 {
		image = sts.Spec.Template.Spec.Containers[0].Image
	}

	return appResponse{
		Name:          sts.Name,
		Namespace:     sts.Namespace,
		Type:          "StatefulSet",
		Replicas:      derefInt32(sts.Spec.Replicas),
		ReadyReplicas: sts.Status.ReadyReplicas,
		PodNames:      pods,
		Labels:        sts.Labels,
		Annotations:   sts.Annotations,
		Env:           extractEnv(sts.Namespace, firstEnv(sts.Spec.Template.Spec.Containers), nil, nil),
		Resources: resourceUsage{
			CPUUsage:   "0",
			MemUsage:   "0",
			CPURequest: requests.cpu.String(),
			MemRequest: requests.mem.String(),
			CPULimit:   limits.cpu.String(),
			MemLimit:   limits.mem.String(),
		},
		Volumes:    volumes,
		Secrets:    secrets,
		ConfigMaps: configMaps,
		Image:      image,
	}
}

func (h *KubeHandler) mapCnpgCluster(ctx context.Context, cluster *cnpgCluster) appResponse {
	pods, _ := h.listPodsForLabel(ctx, cluster.Metadata.Namespace, fmt.Sprintf("%s=%s", cnpgClusterLabelKey, cluster.Metadata.Name))
	requests, limits := sumResourceRequirements(cluster.Spec.Resources)
	image := cluster.Spec.ImageName
	if image == "" {
		image = cluster.Status.Image
	}

	return appResponse{
		Name:          cluster.Metadata.Name,
		Namespace:     cluster.Metadata.Namespace,
		Type:          "Cluster",
		Replicas:      derefInt32(cluster.Spec.Instances),
		ReadyReplicas: cluster.Status.ReadyInstances,
		PodNames:      pods,
		Labels:        cluster.Metadata.Labels,
		Annotations:   cluster.Metadata.Annotations,
		Env:           map[string]string{},
		Resources: resourceUsage{
			CPUUsage:   "0",
			MemUsage:   "0",
			CPURequest: requests.cpu.String(),
			MemRequest: requests.mem.String(),
			CPULimit:   limits.cpu.String(),
			MemLimit:   limits.mem.String(),
		},
		Volumes:    []volumeMountResponse{},
		Secrets:    []string{},
		ConfigMaps: []string{},
		Image:      image,
	}
}

func (h *KubeHandler) mapDragonfly(ctx context.Context, dragonfly *dragonflyResource) appResponse {
	pods, ready := h.listPodsForLabel(ctx, dragonfly.Metadata.Namespace, fmt.Sprintf("%s=%s", dragonflyAppLabelKey, dragonfly.Metadata.Name))
	requests, limits := sumResourceRequirements(dragonfly.Spec.Resources)
	secretRefs, configRefs := extractEnvRefs(dragonfly.Spec.Env)

	return appResponse{
		Name:          dragonfly.Metadata.Name,
		Namespace:     dragonfly.Metadata.Namespace,
		Type:          "Dragonfly",
		Replicas:      derefInt32(dragonfly.Spec.Replicas),
		ReadyReplicas: ready,
		PodNames:      pods,
		Labels:        dragonfly.Metadata.Labels,
		Annotations:   dragonfly.Metadata.Annotations,
		Env:           extractEnv(dragonfly.Metadata.Namespace, dragonfly.Spec.Env, nil, nil),
		Resources: resourceUsage{
			CPUUsage:   "0",
			MemUsage:   "0",
			CPURequest: requests.cpu.String(),
			MemRequest: requests.mem.String(),
			CPULimit:   limits.cpu.String(),
			MemLimit:   limits.mem.String(),
		},
		Volumes:    []volumeMountResponse{},
		Secrets:    secretRefs,
		ConfigMaps: configRefs,
		Image:      dragonfly.Spec.Image,
	}
}

func (h *KubeHandler) listPodsForLabel(ctx context.Context, namespace, selector string) ([]string, int32) {
	if selector == "" {
		return []string{}, 0
	}
	pods, err := h.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return []string{}, 0
	}

	podNames := make([]string, 0, len(pods.Items))
	var ready int32
	for _, pod := range pods.Items {
		if !h.allowPod(&pod) {
			continue
		}
		podNames = append(podNames, pod.Name)
		if podReady(&pod) {
			ready++
		}
	}
	sort.Strings(podNames)
	return podNames, ready
}

func sumResourceRequirements(req corev1.ResourceRequirements) (resourceTotals, resourceTotals) {
	var requests resourceTotals
	var limits resourceTotals
	if quantity, ok := req.Requests[corev1.ResourceCPU]; ok {
		requests.cpu.Add(quantity)
	}
	if quantity, ok := req.Requests[corev1.ResourceMemory]; ok {
		requests.mem.Add(quantity)
	}
	if quantity, ok := req.Limits[corev1.ResourceCPU]; ok {
		limits.cpu.Add(quantity)
	}
	if quantity, ok := req.Limits[corev1.ResourceMemory]; ok {
		limits.mem.Add(quantity)
	}
	return requests, limits
}

func extractEnvRefs(envs []corev1.EnvVar) ([]string, []string) {
	secrets := map[string]struct{}{}
	configMaps := map[string]struct{}{}
	for _, env := range envs {
		if env.ValueFrom == nil {
			continue
		}
		if env.ValueFrom.SecretKeyRef != nil {
			secrets[env.ValueFrom.SecretKeyRef.Name] = struct{}{}
		}
		if env.ValueFrom.ConfigMapKeyRef != nil {
			configMaps[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
		}
	}
	return mapKeys(secrets), mapKeys(configMaps)
}

func podReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, status := range pod.Status.ContainerStatuses {
		if !status.Ready {
			return false
		}
	}
	return true
}

func (h *KubeHandler) listPodsForSelector(ctx context.Context, namespace string, selector *metav1.LabelSelector) []string {
	sel := selectorString(selector)
	if sel == "" {
		return []string{}
	}
	pods, err := h.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return []string{}
	}
	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if !h.allowPod(&pod) {
			continue
		}
		podNames = append(podNames, pod.Name)
	}
	sort.Strings(podNames)
	return podNames
}

func extractVolumeMounts(containers []corev1.Container) []volumeMountResponse {
	seen := map[string]volumeMountResponse{}
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			key := mount.Name + ":" + mount.MountPath
			seen[key] = volumeMountResponse{
				Name:      mount.Name,
				MountPath: mount.MountPath,
				ReadOnly:  mount.ReadOnly,
			}
		}
	}
	volumes := make([]volumeMountResponse, 0, len(seen))
	for _, mount := range seen {
		volumes = append(volumes, mount)
	}
	return volumes
}

func extractSecretsConfigMaps(containers []corev1.Container, volumes []corev1.Volume) ([]string, []string) {
	secrets := map[string]struct{}{}
	configMaps := map[string]struct{}{}

	for _, volume := range volumes {
		if volume.Secret != nil {
			secrets[volume.Secret.SecretName] = struct{}{}
		}
		if volume.ConfigMap != nil {
			configMaps[volume.ConfigMap.Name] = struct{}{}
		}
	}

	for _, container := range containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				secrets[envFrom.SecretRef.Name] = struct{}{}
			}
			if envFrom.ConfigMapRef != nil {
				configMaps[envFrom.ConfigMapRef.Name] = struct{}{}
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.SecretKeyRef != nil {
					secrets[env.ValueFrom.SecretKeyRef.Name] = struct{}{}
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					configMaps[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
				}
			}
		}
	}

	return mapKeys(secrets), mapKeys(configMaps)
}

type resourceTotals struct {
	cpu resource.Quantity
	mem resource.Quantity
}

func sumResourceRequests(containers []corev1.Container) (resourceTotals, resourceTotals) {
	var req resourceTotals
	var lim resourceTotals
	for _, container := range containers {
		if quantity, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			req.cpu.Add(quantity)
		}
		if quantity, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			req.mem.Add(quantity)
		}
		if quantity, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			lim.cpu.Add(quantity)
		}
		if quantity, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			lim.mem.Add(quantity)
		}
	}
	return req, lim
}

func mapKeys(input map[string]struct{}) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatAge(created time.Time) string {
	duration := time.Since(created)
	if duration < time.Minute {
		return "<1m"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd", int(duration.Hours()/24))
}

func ownerRefName(refs []metav1.OwnerReference) string {
	for _, ref := range refs {
		if ref.Kind == "ReplicaSet" || ref.Kind == "StatefulSet" || ref.Kind == "Deployment" {
			return ref.Name
		}
	}
	return ""
}

func hasOwnerKind(refs []metav1.OwnerReference, kind string) bool {
	for _, ref := range refs {
		if ref.Kind == kind {
			return true
		}
	}
	return false
}

func derefInt32(val *int32) int32 {
	if val == nil {
		return 0
	}
	return *val
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

func firstEnv(containers []corev1.Container) []corev1.EnvVar {
	if len(containers) == 0 {
		return nil
	}
	return containers[0].Env
}

func extractEnv(namespace string, envs []corev1.EnvVar, user *auth.User, client *kubernetes.Clientset) map[string]string {
	result := map[string]string{}
	for _, env := range envs {
		if env.Value != "" {
			result[env.Name] = env.Value
			continue
		}
		if env.ValueFrom == nil {
			continue
		}
		if env.ValueFrom.SecretKeyRef != nil {
			if user != nil && user.AllowedSecrets && client != nil {
				value, err := fetchSecretValue(client, namespace, env.ValueFrom.SecretKeyRef.Name, env.ValueFrom.SecretKeyRef.Key)
				if err == nil {
					result[env.Name] = value
					continue
				}
			}
			result[env.Name] = "********"
			continue
		}
		if env.ValueFrom.ConfigMapKeyRef != nil {
			if client != nil {
				value, err := fetchConfigMapValue(client, namespace, env.ValueFrom.ConfigMapKeyRef.Name, env.ValueFrom.ConfigMapKeyRef.Key)
				if err == nil {
					result[env.Name] = value
					continue
				}
			}
			result[env.Name] = "********"
		}
	}
	return result
}

func fetchSecretValue(client *kubernetes.Clientset, namespace, name, key string) (string, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value, ok := secret.Data[key]
	if !ok {
		return "", errors.New("secret key not found")
	}
	return string(value), nil
}

func fetchConfigMapValue(client *kubernetes.Clientset, namespace, name, key string) (string, error) {
	cfg, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value, ok := cfg.Data[key]
	if !ok {
		return "", errors.New("configmap key not found")
	}
	return value, nil
}

func (h *KubeHandler) fetchPodMetrics(ctx context.Context, namespace, name string) (resourceUsage, error) {
	path := fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods/%s", namespace, name)
	data, err := h.client.RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		return resourceUsage{}, err
	}

	var payload struct {
		Containers []struct {
			Usage map[string]string `json:"usage"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return resourceUsage{}, err
	}

	cpu := resource.Quantity{}
	mem := resource.Quantity{}
	for _, container := range payload.Containers {
		if val, ok := container.Usage["cpu"]; ok {
			if qty, err := resource.ParseQuantity(val); err == nil {
				cpu.Add(qty)
			}
		}
		if val, ok := container.Usage["memory"]; ok {
			if qty, err := resource.ParseQuantity(val); err == nil {
				mem.Add(qty)
			}
		}
	}

	return resourceUsage{
		CPUUsage: cpu.String(),
		MemUsage: mem.String(),
	}, nil
}

func (h *KubeHandler) listCnpgClusters(ctx context.Context, namespace string) ([]cnpgCluster, error) {
	path := fmt.Sprintf(cnpgClusterListPathFmt, namespace)
	data, err := h.client.RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var list cnpgClusterList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (h *KubeHandler) getCnpgCluster(ctx context.Context, namespace, name string) (*cnpgCluster, error) {
	path := fmt.Sprintf(cnpgClusterGetPathFmt, namespace, name)
	data, err := h.client.RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var cluster cnpgCluster
	if err := json.Unmarshal(data, &cluster); err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (h *KubeHandler) listDragonflies(ctx context.Context, namespace string) ([]dragonflyResource, error) {
	path := fmt.Sprintf(dragonflyListPathFmt, namespace)
	data, err := h.client.RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var list dragonflyList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (h *KubeHandler) getDragonfly(ctx context.Context, namespace, name string) (*dragonflyResource, error) {
	path := fmt.Sprintf(dragonflyGetPathFmt, namespace, name)
	data, err := h.client.RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var dragonfly dragonflyResource
	if err := json.Unmarshal(data, &dragonfly); err != nil {
		return nil, err
	}
	return &dragonfly, nil
}

func (h *KubeHandler) listPodsForApp(ctx context.Context, namespace, name string) ([]string, error) {
	selector, err := h.appSelector(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if selector == "" {
		return nil, errAppNotFound
	}
	pods, err := h.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}
	return podNames, nil
}

func selectorString(selector *metav1.LabelSelector) string {
	if selector == nil {
		return ""
	}
	if s, err := metav1.LabelSelectorAsSelector(selector); err == nil {
		return s.String()
	}
	return ""
}

func (h *KubeHandler) appSelector(ctx context.Context, namespace, name string) (string, error) {
	cluster, err := h.getCnpgCluster(ctx, namespace, name)
	if err == nil {
		return fmt.Sprintf("%s=%s", cnpgClusterLabelKey, cluster.Metadata.Name), nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	dragonfly, err := h.getDragonfly(ctx, namespace, name)
	if err == nil {
		return fmt.Sprintf("%s=%s", dragonflyAppLabelKey, dragonfly.Metadata.Name), nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	deployment, err := h.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return selectorString(deployment.Spec.Selector), nil
	}

	statefulSet, err := h.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return selectorString(statefulSet.Spec.Selector), nil
	}

	return "", errAppNotFound
}
