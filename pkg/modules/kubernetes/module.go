package kubernetes

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/edgeopslabs/nexus/pkg/config"
	"github.com/edgeopslabs/nexus/pkg/registry"
	"github.com/edgeopslabs/nexus/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	moduleName         = "kubernetes"
	listPodsTool       = "k8s_list_pods"
	logsTool           = "k8s_get_logs"
	listNamespacesTool = "k8s_list_namespaces"
	listPodsAllTool    = "k8s_list_pods_all"
)

type Module struct {
	cfg *config.Config
}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return moduleName
}

func (m *Module) Enabled(cfg *config.Config) bool {
	return cfg.Modules.Kubernetes.Enabled
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	if !cfg.Modules.Kubernetes.Enabled {
		slog.Info("kubernetes module disabled by config")
	}
	return nil
}

func (m *Module) GetTools() []mcp.Tool {
	if m.cfg == nil || !m.cfg.Modules.Kubernetes.Enabled {
		return nil
	}

	return []mcp.Tool{
		mcp.NewTool(listNamespacesTool,
			mcp.WithDescription("List all namespaces in the cluster."),
			mcp.WithNumber("max_namespaces", mcp.Description("Max namespaces to return (default 200, max 1000).")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(listPodsTool,
			mcp.WithDescription("List all pods in a specific namespace. Use this to check app health."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("The namespace to query (e.g., 'default', 'kube-system')")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(listPodsAllTool,
			mcp.WithDescription("List pods across all namespaces, optionally filtering to erroring pods."),
			mcp.WithBoolean("error_only", mcp.Description("Only include pods with error conditions (default true).")),
			mcp.WithNumber("max_pods", mcp.Description("Max pods to return (default 200, max 1000).")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		mcp.NewTool(logsTool,
			mcp.WithDescription("Fetch error-focused logs for a pod or workload (deployment, daemonset, statefulset, job)."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (e.g., 'default').")),
			mcp.WithString("kind", mcp.Required(), mcp.Description("Resource kind: pod, deployment, daemonset, statefulset, job.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Resource name (pod/workload).")),
			mcp.WithString("container", mcp.Description("Container name (optional). If empty, all containers are included.")),
			mcp.WithNumber("tail_lines", mcp.Description("Max log lines per container (default 200, max 500).")),
			mcp.WithNumber("since_seconds", mcp.Description("Only return logs newer than this many seconds.")),
			mcp.WithBoolean("previous", mcp.Description("Return logs from the previous container instance if it crashed.")),
			mcp.WithString("contains", mcp.Description("Filter logs to lines containing this string (case-insensitive).")),
			mcp.WithBoolean("error_only", mcp.Description("Only include common error patterns (recommended to reduce token usage).")),
			mcp.WithNumber("event_limit", mcp.Description("Max events to include per pod (default 5, max 20).")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
	}
}

func (m *Module) HandleCall(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.cfg.Modules.Kubernetes.Enabled {
		return mcp.NewToolResultError("kubernetes module is disabled"), nil
	}

	switch name {
	case listNamespacesTool:
		return m.handleListNamespaces(ctx, args)
	case listPodsTool:
		return m.handleListPods(ctx, args)
	case listPodsAllTool:
		return m.handleListPodsAll(ctx, args)
	case logsTool:
		return m.handleLogs(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

func (m *Module) handleListNamespaces(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	maxNamespaces := clampInt(getIntArg(args, "max_namespaces", 200), 1, 1000)

	clientset, err := m.getClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("k8s auth failed: %v", err)), nil
	}
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list namespaces: %v", err)), nil
	}

	count := len(namespaces.Items)
	if count == 0 {
		return mcp.NewToolResultText("No namespaces found."), nil
	}

	if count > maxNamespaces {
		namespaces.Items = namespaces.Items[:maxNamespaces]
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Namespaces (showing %d of %d):\n", len(namespaces.Items), count))
	for _, ns := range namespaces.Items {
		output.WriteString("- " + ns.Name + "\n")
	}
	return mcp.NewToolResultText(output.String()), nil
}

func (m *Module) handleListPods(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	namespace, _ := args["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	clientset, err := m.getClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("k8s auth failed: %v", err)), nil
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list pods: %v", err)), nil
	}

	var output string
	if len(pods.Items) == 0 {
		output = fmt.Sprintf("No pods found in namespace '%s'.", namespace)
	} else {
		for _, pod := range pods.Items {
			restarts := 0
			if len(pod.Status.ContainerStatuses) > 0 {
				restarts = int(pod.Status.ContainerStatuses[0].RestartCount)
			}
			output += fmt.Sprintf("Pod: %s | Status: %s | Restarts: %d\n",
				pod.Name, pod.Status.Phase, restarts)
		}
	}

	return mcp.NewToolResultText(output), nil
}

func (m *Module) handleListPodsAll(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	errorOnly := getBoolArg(args, "error_only", true)
	maxPods := clampInt(getIntArg(args, "max_pods", 200), 1, 1000)

	clientset, err := m.getClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("k8s auth failed: %v", err)), nil
	}

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list pods across namespaces: %v", err)), nil
	}
	if len(pods.Items) == 0 {
		return mcp.NewToolResultText("No pods found."), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Pods across all namespaces (error_only=%t):\n", errorOnly))

	count := 0
	for _, pod := range pods.Items {
		if errorOnly && !podHasErrors(&pod) {
			continue
		}
		count++
		summary := podErrorSummary(&pod)
		line := fmt.Sprintf("- %s/%s | phase=%s", pod.Namespace, pod.Name, pod.Status.Phase)
		if summary != "" {
			line += " | " + summary
		}
		output.WriteString(line + "\n")
		if count >= maxPods {
			output.WriteString(fmt.Sprintf("... truncated at %d pods\n", maxPods))
			break
		}
	}

	if count == 0 {
		return mcp.NewToolResultText("No erroring pods found."), nil
	}
	return mcp.NewToolResultText(output.String()), nil
}

func (m *Module) handleLogs(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	namespace := getStringArg(args, "namespace", "default")
	kind := strings.ToLower(getStringArg(args, "kind", "pod"))
	name := getStringArg(args, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	container := getStringArg(args, "container", "")
	tailLines := clampInt(getIntArg(args, "tail_lines", 200), 1, 500)
	sinceSeconds := getIntArg(args, "since_seconds", 0)
	previous := getBoolArg(args, "previous", false)
	contains := strings.TrimSpace(getStringArg(args, "contains", ""))
	errorOnly := getBoolArg(args, "error_only", true)
	eventLimit := clampInt(getIntArg(args, "event_limit", 5), 1, 20)

	clientset, err := m.getClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("k8s auth failed: %v", err)), nil
	}

	var pods []corev1.Pod
	switch kind {
	case "pod", "pods":
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get pod %s: %v", name, err)), nil
		}
		pods = []corev1.Pod{*pod}
	case "deployment", "deploy", "deployments":
		deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get deployment %s: %v", name, err)), nil
		}
		pods, err = m.listPodsForSelector(ctx, namespace, deploy.Spec.Selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list pods for deployment %s: %v", name, err)), nil
		}
	case "daemonset", "ds", "daemonsets":
		ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get daemonset %s: %v", name, err)), nil
		}
		pods, err = m.listPodsForSelector(ctx, namespace, ds.Spec.Selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list pods for daemonset %s: %v", name, err)), nil
		}
	case "statefulset", "sts", "statefulsets":
		sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get statefulset %s: %v", name, err)), nil
		}
		pods, err = m.listPodsForSelector(ctx, namespace, sts.Spec.Selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list pods for statefulset %s: %v", name, err)), nil
		}
	case "job", "jobs":
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get job %s: %v", name, err)), nil
		}
		pods, err = m.listPodsForSelector(ctx, namespace, job.Spec.Selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list pods for job %s: %v", name, err)), nil
		}
	default:
		return mcp.NewToolResultError("kind must be one of: pod, deployment, daemonset, statefulset, job"), nil
	}

	if len(pods) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No pods found for %s/%s in namespace %s.", kind, name, namespace)), nil
	}

	var output strings.Builder
	for _, pod := range pods {
		output.WriteString(fmt.Sprintf("=== Pod: %s | Phase: %s ===\n", pod.Name, pod.Status.Phase))
		if status := summarizePodStatus(&pod); status != "" {
			output.WriteString(status + "\n")
		}
		if events := m.fetchPodEvents(ctx, clientset, namespace, pod.Name, eventLimit); events != "" {
			output.WriteString("Events:\n")
			output.WriteString(events + "\n")
		}

		containers := resolveContainers(&pod, container)
		for _, c := range containers {
			logs, err := m.fetchPodLogs(ctx, clientset, namespace, pod.Name, c, tailLines, sinceSeconds, previous)
			if err != nil {
				output.WriteString(fmt.Sprintf("[container %s] log error: %v\n", c, err))
				continue
			}
			filtered := filterLogLines(logs, contains, errorOnly)
			if filtered == "" {
				filtered = "(no matching log lines)"
			}
			output.WriteString(fmt.Sprintf("[container %s] tail=%d since=%ds previous=%t\n", c, tailLines, sinceSeconds, previous))
			output.WriteString(filtered + "\n")
		}
		output.WriteString("\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (m *Module) getClient() (*kubernetes.Clientset, error) {
	kubeconfig := resolveKubeconfig(m.cfg.Modules.Kubernetes.Kubeconfig)
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func (m *Module) listPodsForSelector(ctx context.Context, namespace string, selector *metav1.LabelSelector) ([]corev1.Pod, error) {
	if selector == nil {
		return nil, fmt.Errorf("selector not defined")
	}
	labelsSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	list, err := m.getClient()
	if err != nil {
		return nil, err
	}
	pods, err := list.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelsSelector.String(),
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (m *Module) fetchPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, container string, tailLines int, sinceSeconds int, previous bool) (string, error) {
	options := &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
	}
	if tailLines > 0 {
		lines := int64(tailLines)
		options.TailLines = &lines
	}
	if sinceSeconds > 0 {
		seconds := int64(sinceSeconds)
		options.SinceSeconds = &seconds
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, options)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func summarizePodStatus(pod *corev1.Pod) string {
	var lines []string
	if pod.Status.Reason != "" {
		lines = append(lines, fmt.Sprintf("Reason: %s", pod.Status.Reason))
	}
	if pod.Status.Message != "" {
		lines = append(lines, fmt.Sprintf("Message: %s", pod.Status.Message))
	}
	for _, status := range pod.Status.ContainerStatuses {
		state := "unknown"
		if status.State.Waiting != nil {
			state = fmt.Sprintf("waiting(%s)", status.State.Waiting.Reason)
		} else if status.State.Terminated != nil {
			state = fmt.Sprintf("terminated(%s)", status.State.Terminated.Reason)
		} else if status.State.Running != nil {
			state = "running"
		}
		lines = append(lines, fmt.Sprintf("Container %s: state=%s restarts=%d", status.Name, state, status.RestartCount))
	}
	return strings.Join(lines, "\n")
}

func podHasErrors(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodPending {
		return true
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			return true
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > 0 {
			return true
		}
		if status.State.Waiting != nil && status.State.Waiting.Reason != "" {
			return true
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return true
		}
	}
	return false
}

func podErrorSummary(pod *corev1.Pod) string {
	var parts []string
	if pod.Status.Reason != "" {
		parts = append(parts, pod.Status.Reason)
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			parts = append(parts, "not-ready")
			break
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason != "" {
			parts = append(parts, fmt.Sprintf("%s:waiting(%s)", status.Name, status.State.Waiting.Reason))
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			parts = append(parts, fmt.Sprintf("%s:exit(%d)", status.Name, status.State.Terminated.ExitCode))
		}
		if status.RestartCount > 0 {
			parts = append(parts, fmt.Sprintf("%s:restarts(%d)", status.Name, status.RestartCount))
		}
	}
	return strings.Join(uniqueStrings(parts), ", ")
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func resolveContainers(pod *corev1.Pod, specified string) []string {
	if specified != "" {
		return []string{specified}
	}
	var containers []string
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}
	return containers
}

func filterLogLines(logs string, contains string, errorOnly bool) string {
	if contains == "" && !errorOnly {
		return logs
	}
	needle := strings.ToLower(contains)
	lines := strings.Split(logs, "\n")
	var filtered []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if contains != "" && !strings.Contains(lower, needle) {
			continue
		}
		if errorOnly && !matchesErrorPattern(lower) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func matchesErrorPattern(line string) bool {
	patterns := []string{
		"error",
		"failed",
		"panic",
		"fatal",
		"exception",
		"crash",
		"backoff",
		"oom",
		"terminated",
		"refused",
		"timeout",
	}
	for _, pattern := range patterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

func (m *Module) fetchPodEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, limit int) string {
	selector := fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", podName)
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		return fmt.Sprintf("failed to fetch events: %v", err)
	}
	if len(events.Items) == 0 {
		return ""
	}

	sort.Slice(events.Items, func(i, j int) bool {
		return eventTime(events.Items[i]).After(eventTime(events.Items[j]))
	})

	if limit > 0 && len(events.Items) > limit {
		events.Items = events.Items[:limit]
	}

	var lines []string
	for _, event := range events.Items {
		timestamp := eventTime(event).Format(time.RFC3339)
		line := fmt.Sprintf("- [%s] %s %s (count=%d): %s", timestamp, event.Type, event.Reason, event.Count, event.Message)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func eventTime(event corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func getStringArg(args map[string]interface{}, key, def string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return def
}

func getIntArg(args map[string]interface{}, key string, def int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if v != "" {
				var n int
				_, err := fmt.Sscanf(v, "%d", &n)
				if err == nil {
					return n
				}
			}
		}
	}
	return def
}

func getBoolArg(args map[string]interface{}, key string, def bool) bool {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			switch strings.ToLower(v) {
			case "true", "1", "yes", "y":
				return true
			case "false", "0", "no", "n":
				return false
			}
		}
	}
	return def
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func resolveKubeconfig(path string) string {
	if path == "" {
		if home := homedir.HomeDir(); home != "" {
			return filepath.Join(home, ".kube", "config")
		}
		return ""
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	return path
}

func init() {
	registry.Register(moduleName, New())
}

var _ types.NexusModule = (*Module)(nil)
