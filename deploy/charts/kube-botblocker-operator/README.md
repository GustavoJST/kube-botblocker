# kube-botblocker-operator

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

Easily configure User-Agent blocks for selected ingresses - ingress-nginx only

**Homepage:** <https://github.com/GustavoJST/kube-botblocker>

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

2. Use `--set crds.enabled=true` when installing the chart. This will make chart install and manage the necessary CRDs, aswell as updating it when the chart updates.

   This is not the default as to prevent **data loss**. Read more about it in `crds.enable` in the `values.yaml` table below.

## Uninstalling

Whether you choose the first or second option above on install, if you wish to uninstall kube-botblocker and its configurations present inside ingresses,
run the command below **BEFORE** uninstalling the ingress:

```bash
kubectl annotate -all -A kube-botblocker.github.io/protectedIngress- kube-botblocker.github.io/ingressConfigName-
```

kube-botblocker will remove its configuration from ingresses that were annotated. Give it some time and then continue with the `helm uninstall` command.

Not doing the process mentioned above will leave you with ingresses that have kube-botblocker related annotations and configuration **even after the chart is uninstalled**.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity to add to the controller Pod |
| crds.enabled | bool | `false` | Whether the helm chart should create and update the CRDs. It is false by default, which implies that the CRDs must be managed independently with the kube-botblocker-operator-crds helm chart. **WARNING**: If set to true, uninstalling the chart (or doing a helm upgrade after setting it back to false) will cause all CRDs and custom resources (IngressConfigs) to be DELETED, causing DATA LOSS |
| currentNamespaceOnly | bool | `false` | Whether the operator should watch Ingress resources only in its own namespace or not |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname |
| image.pullPolicy | string | `"IfNotPresent"` | Sets the pull policy for the controller image |
| image.repository | string | `"quay.io/gustavojst/kube-botblocker"` | Repository path to the controller image |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Image pull secrets for pulling images from the registry |
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
| podSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Security context to add to controller Pod |
| rbac.enabled | bool | `true` | Creates the necessary RBAC resources |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":8081},"initialDelaySeconds":5,"periodSeconds":10}` | readinessProbe to add to the controller container |
| resources | object | `{}` | Resources to add to controller container |
| securityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context to add to controller container |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` | Tolerations to add to the controller Pod |

----------------------------------------------

Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs/).