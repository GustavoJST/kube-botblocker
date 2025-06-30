# kube-botblocker-operator

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.2.1](https://img.shields.io/badge/Version-0.2.1-informational?style=flat-square) ![AppVersion: 0.2.0](https://img.shields.io/badge/AppVersion-0.2.0-informational?style=flat-square)

Easily configure User-Agent blocks for selected ingresses - ingress-nginx only

**Homepage:** <https://gustavojst.github.io/kube-botblocker>

## Source Code

* <https://github.com/GustavoJST/kube-botblocker>

## Installing

There are two ways to install and manage the kube-botblocker helm chart:

1. Recommended:
    Install the kube-botblocker CRDs chart separately and then install this helm chart. When this chart is uninstalled, the CRDs will remain as long as the CRDs chart is not uninstalled, preventing data loss.

    ```bash
    helm repo add kube-botblocker https://gustavojst.github.io/kube-botblocker
    helm install kube-botblocker-operator-crds kube-botblocker/kube-botblocker-operator-crds
    helm install kube-botblocker-operator kube-botblocker/kube-botblocker-operator
    ```

2. Use `--set crds.enabled=true` when installing the chart. This will make the chart install and manage the necessary CRDs, aswell as updating it when the chart updates.

   This is not the default as to prevent accidental **data loss**. Read more about it in `crds.enable` in the `values.yaml` table below.

## Uninstalling

Whether you choose the first or second option above on install, if you wish to uninstall kube-botblocker, you can:

1. Simply uninstall the helm chart if `.cleanupJob.enabled: true` (the default). This will run a pre-delete helm hook Job that will uninstall all kube-botblocker related,
configuration (including IngressConfig objects and configuration inside associated ingresses).

2. If `.cleanupJob.enabled: false`, run the command below **BEFORE** uninstalling the helm chart:

    ```bash
    kubectl annotate --all -A kube-botblocker.github.io/ingressConfigName-
    ```

    Confirm all configuration from associated ingresses have been removed and delete any IngressConfig objects left. Then, proceed to chart removal with `helm uninstall`

Not doing the process mentioned above will leave you with dangling IngressConfig objects and ingresses that have kube-botblocker related annotations and configuration **even after the chart is uninstalled**.

Lastly, if `crds.enabled: false`, remove the kube-botblocker CRDs helm chart.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity to add to the controller Pod |
| cleanupJob.affinity | object | `{}` | Assign custom affinity rules to the cleanup job |
| cleanupJob.annotations | object | `{}` | Defines annotations to add to the cleanup job |
| cleanupJob.containerSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Defines container-level security context configuration for the cleanup job |
| cleanupJob.enabled | bool | `true` | Wheter to run a cleanup job do remove all IngressConfig and their configuration present on associated Ingresses on helm chart removal |
| cleanupJob.env | object | `{}` | Defines the pullPolicy for the cleanup job image |
| cleanupJob.image.imagePullSecrets | list | `[]` | imagePullSecret for cleanup job |
| cleanupJob.image.kubectl.pullPolicy | string | `"IfNotPresent"` | Defines the pullPolicy for the cleanup job image |
| cleanupJob.image.kubectl.registry | string | `"registry.k8s.io"` | Defines the registry used to pull the image for the cleanup job |
| cleanupJob.image.kubectl.repository | string | `"kubectl"` | Defines the repository used to pull the image for the cleanup job |
| cleanupJob.image.kubectl.sha | string | `""` | Defines the sha256 to be used during image pull for the cleanup job. Useful for sha256 pinning. An empty value will not use the sha256 during the image pull process |
| cleanupJob.image.kubectl.tag | string | `""` | Defines the image tag used to pull the image for the cleanup job. An empty value will default to the current Kubernetes version |
| cleanupJob.labels | object | `{}` | Defines labels to add to the cleanup job |
| cleanupJob.nodeSelector | object | `{}` | Defines node selector for cleanup job |
| cleanupJob.podAnnotations | object | `{}` | Defines annotations to add to the cleanup job pod |
| cleanupJob.podLabels | object | `{}` | Defines labels to add to the cleanup job pod |
| cleanupJob.podSecurityContext | object | `{"fsGroup":65534,"runAsGroup":65534,"runAsNonRoot":true,"runAsUser":65534,"seccompProfile":{"type":"RuntimeDefault"}}` | Defines pod-level security context configuration for the cleanup job |
| cleanupJob.resources | object | `{}` | Defines resources requests and limits for cleanup job pod |
| cleanupJob.serviceAccount.annotations | object | `{}` | Defines annotations for the cleanup job service account |
| cleanupJob.serviceAccount.labels | object | `{}` | Defines labels for the cleanup job service account |
| cleanupJob.tolerations | list | `[]` | Defines tolerations for the cleanup job |
| crds.enabled | bool | `false` | Whether the helm chart should create and update the CRDs. It is false by default, which implies that the CRDs must be managed independently with the kube-botblocker-operator-crds helm chart. **WARNING**: If set to true, uninstalling the chart (or doing a helm upgrade after setting it back to false) will cause all CRDs and custom resources (IngressConfigs) to be DELETED, causing DATA LOSS |
| currentNamespaceOnly | bool | `false` | Whether the operator should watch Ingress resources only in its own namespace or not |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname |
| image.pullPolicy | string | `"IfNotPresent"` | Sets the pull policy for the controller image |
| image.repository | string | `"quay.io/gustavojst/kube-botblocker"` | Repository path to the controller image |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Image pull secrets for pulling images from the registry |
| ingressConfigs | list | `[]` | List of IngressConfig resources to be created with the helm chart. Note that if .cleanupJob.enabled is false, these resources will not be outright deleted when the chart is uninstalled due to the presence of finalizers. You can either wait for the deletionTimestamp of each object to expire or perform a manual cleanup. |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":8081},"initialDelaySeconds":15,"periodSeconds":20}` | livenessProbe to add to the controller container |
| metrics.enabled | bool | `false` | Enables exposure of the operator internal metrics in prometheus format |
| metrics.port | int | `8443` | Configures the operator metrics port |
| metrics.serviceMonitor.additionalLabels | object | `{}` | Additional labels to be added to the created ServiceMonitor |
| metrics.serviceMonitor.enabled | bool | `true` | Creates a ServiceMonitor object for Prometheus to scrape |
| metrics.serviceMonitor.interval | string | `"30s"` | Interval to scrape metrics |
| metrics.serviceMonitor.metricRelabelings | list | `[]` | MetricRelabelConfigs to apply to samples before ingestion |
| metrics.serviceMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping |
| metrics.serviceMonitor.scrapeTimeout | string | `"25s"` | Timeout if metrics can't be retrieved in given time interval |
| nameOverride | string | `""` | Overrides the chart's name. |
| nodeSelector | object | `{}` | Node selectors to add to the controller Pod |
| podAnnotations | object | `{}` | Annotations to add to the controller Pod |
| podLabels | object | `{}` | Labels to add to the controller Pod |
| podSecurityContext | object | `{}` | Security context to add to controller Pod |
| rbac.enabled | bool | `true` | Creates the necessary RBAC resources |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":8081},"initialDelaySeconds":5,"periodSeconds":10}` | readinessProbe to add to the controller container |
| resources | object | `{}` | Resources to add to controller container |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context to add to controller container |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` | Tolerations to add to the controller Pod |

----------------------------------------------

Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs/).