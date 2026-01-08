package config

// DefaultIgnorePaths returns the built-in list of field paths to ignore.
// These are fields that Kubernetes automatically adds or manages,
// which would cause false-positive drift detection.
//
// Categories:
//   - Metadata: Server-assigned fields (uid, resourceVersion, etc.)
//   - Status: Always server-managed
//   - Deployment defaults: K8s adds these if not specified
//   - Pod spec defaults: Container runtime defaults
//   - Service defaults: Cluster networking defaults
//   - StatefulSet defaults: Storage and ordering defaults
//   - HPA defaults: Autoscaler controller annotations
func DefaultIgnorePaths() []string {
	return []string{
		// ─────────────────────────────────────────────────────────────────
		// Metadata (server-assigned, changes on every update)
		// ─────────────────────────────────────────────────────────────────
		"metadata.resourceVersion",
		"metadata.uid",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.managedFields",
		"metadata.selfLink",

		// Controller-managed annotations
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
		"metadata.annotations.deployment.kubernetes.io/revision",
		"metadata.annotations.autoscaling.alpha.kubernetes.io/conditions",
		"metadata.annotations.autoscaling.alpha.kubernetes.io/current-metrics",

		// Helm release tracking (added by helm install/upgrade, not in helm template)
		"metadata.annotations.meta.helm.sh/release-name",
		"metadata.annotations.meta.helm.sh/release-namespace",
		"metadata.labels.app.kubernetes.io/managed-by",

		// ─────────────────────────────────────────────────────────────────
		// Status (always server-side, never in manifests)
		// ─────────────────────────────────────────────────────────────────
		"status",

		// ─────────────────────────────────────────────────────────────────
		// Deployment defaults (K8s adds these if not specified)
		// ─────────────────────────────────────────────────────────────────
		"spec.progressDeadlineSeconds",
		"spec.revisionHistoryLimit",
		"spec.strategy",
		"spec.template.metadata.creationTimestamp",

		// ─────────────────────────────────────────────────────────────────
		// Pod spec defaults (container runtime defaults)
		// ─────────────────────────────────────────────────────────────────
		"spec.template.spec.dnsPolicy",
		"spec.template.spec.restartPolicy",
		"spec.template.spec.schedulerName",
		"spec.template.spec.terminationGracePeriodSeconds",
		"spec.template.spec.securityContext",

		// ─────────────────────────────────────────────────────────────────
		// Container defaults
		// ─────────────────────────────────────────────────────────────────
		"spec.template.spec.containers.imagePullPolicy",
		"spec.template.spec.containers.terminationMessagePath",
		"spec.template.spec.containers.terminationMessagePolicy",
		"spec.template.spec.containers.ports.protocol",

		// Probe defaults
		"spec.template.spec.containers.livenessProbe.failureThreshold",
		"spec.template.spec.containers.livenessProbe.successThreshold",
		"spec.template.spec.containers.livenessProbe.timeoutSeconds",
		"spec.template.spec.containers.livenessProbe.httpGet.scheme",
		"spec.template.spec.containers.readinessProbe.failureThreshold",
		"spec.template.spec.containers.readinessProbe.successThreshold",
		"spec.template.spec.containers.readinessProbe.timeoutSeconds",
		"spec.template.spec.containers.readinessProbe.httpGet.scheme",

		// ─────────────────────────────────────────────────────────────────
		// Service defaults (cluster networking)
		// ─────────────────────────────────────────────────────────────────
		"spec.clusterIP",
		"spec.clusterIPs",
		"spec.internalTrafficPolicy",
		"spec.ipFamilies",
		"spec.ipFamilyPolicy",
		"spec.sessionAffinity",
		"spec.ports.protocol",

		// ─────────────────────────────────────────────────────────────────
		// StatefulSet defaults
		// ─────────────────────────────────────────────────────────────────
		"spec.persistentVolumeClaimRetentionPolicy",
	}
}
