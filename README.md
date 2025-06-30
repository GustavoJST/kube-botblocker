[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kube-botblocker)](https://artifacthub.io/packages/search?repo=kube-botblocker)

# kube-botblocker

kube-botblocker is a operator that simplifies User-Agent blocking for your ingress-nginx Ingresses.

## Getting Started

### Prerequisites
- ingress-nginx controller present in the cluster
- `allow-snippet-annotations` must be set to `true`. You can set it either on the ingress-nginx controller [configmap](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/#allow-snippet-annotations) or through the [helm chart](https://artifacthub.io/packages/helm/ingress-nginx/ingress-nginx?modal=values&path=controller.allowSnippetAnnotations)
- For ingress-nginx >= 1.12.0, it is also necessary to set `annotations-risk-level` to `Critical`, configurable only through the ingress-nginx [configmap](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/#annotations-risk-level)
  - This is because the [server-snippet](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#server-snippet) annotation used by kube-botblocker is [classified as Critical](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations-risk/), while the default allowed risk level for annotations was decreased from `Critical` to `High` on 1.12.0

### Installing/Uninstalling
The operator can be installed using the provided helm chart. Documentation on how to install/uninstall and available parameters of the `values.yaml` can be found here [here](https://github.com/GustavoJST/kube-botblocker/tree/main/deploy/charts/kube-botblocker-operator).


## How to use it
kube-botblocker introduces a new custom resource called `IngressConfig`, in which you can specify a list of user agents to be blocked on a per Ingress level:
```yaml
apiVersion: kube-botblocker.github.io/v1alpha1
kind: IngressConfig
metadata:
  name: useragent-blocklist
spec:
  blockedUserAgents:
    - AI2Bot
    - Ai2Bot-Dolma
    - Amazonbot
    - anthropic-ai
    - Applebot
    - Applebot-Extended
    - Bytespider
    - CCBot
    - ChatGPT-User
    - Claude-Web
    - ClaudeBot
    - cohere-ai
    - Diffbot
    - DuckAssistBot
    - FacebookBot
    - facebookexternalhit
    - FriendlyCrawler
    - Google-Extended
    - GoogleOther
    - GoogleOther-Image
    - GoogleOther-Video
    - GPTBot
    - iaskspider/2.0
    - ICCCrawler
    - ImagesiftBot
    - img2dataset
    - ISSCyberRiskCrawler
    - KangarooBot
    - Meta-ExternalAgent
    - Meta-ExternalFetcher
    - OAI-SearchBot
    - omgili
    - omgilibot
    - PerplexityBot
    - PetalBot
    - Scrapy
    - SidetradeIndexerBot
    - Timpibot
    - VelenPublicWebCrawler
    - Webzio-Extended
    - YouBot
    - AhrefsBot
    - SemrushBot
    - meta-externalagent
```
>**NOTE**: The IngressConfig custom resource must reside in the same namespace where kube-botblocker is running, even if `CurrentNamespaceOnly` is set to `false` in the [helm chart](#deployment-modes).

>**NOTE²**: User agents inside `blockedUserAgents` are matched using a **case insensitive** strategy (NGINX ~* operator).
>
>For example, the `AhrefsBot` user agent in the IngressConfig above will match the user-agent string `Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)`, since one `AhrefsBot` is present in the user-agent string.

After the IngressConfig custom resource is created, you can reference it using the annotations below inside a Ingress you to protect:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
  annotations:
    # Required - Specify the name of the IngressConfig you want to use
    kube-botblocker.github.io/ingressConfigName: "useragent-blocklist"
spec:
  rules:
  # ...rest of your Ingress configuration....
```

When the annotation is set, kube-botblocker will generate the required configuration and either append it to the existing `nginx.ingress.kubernetes.io/server-snippet` annotation or create it if it doesn’t already exist:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  annotations:
    kube-botblocker.github.io/ingressConfigName: useragent-blocklist
    kube-botblocker.github.io/ingressConfigSpecHash: 6ba9a2e583e163a90764393df1bcd8695fca8558c0dbcbe8d4524eeeb24346fe
    nginx.ingress.kubernetes.io/server-snippet: |-
      # kube-botblocker.github.io operator: Configuration start
      # Configuration added by kube-botblocker operator. Do not edit any of this manually
      if ($http_user_agent ~* "(AI2Bot|Ai2Bot-Dolma|Amazonbot|anthropic-ai|Applebot|Applebot-Extended|Bytespider|CCBot|ChatGPT-User|Claude-Web|ClaudeBot|cohere-ai|Diffbot|DuckAssistBot|FacebookBot|facebookexternalhit|FriendlyCrawler|Google-Extended|GoogleOther|GoogleOther-Image|GoogleOther-Video|GPTBot|iaskspider/2.0|ICCCrawler|ImagesiftBot|img2dataset|ISSCyberRiskCrawler|KangarooBot|Meta-ExternalAgent|Meta-ExternalFetcher|OAI-SearchBot|omgili|omgilibot|PerplexityBot|PetalBot|Scrapy|SidetradeIndexerBot|Timpibot|VelenPublicWebCrawler|Webzio-Extended|YouBot|AhrefsBot|SemrushBot|meta-externalagent)") {
        return 403;
      }
      # kube-botblocker.github.io operator: Configuration end
spec:
  rules:
  # ...rest of your Ingress configuration....
```

> **NOTE**: As said in the generated configuration above, **do not remove or edit the generated configuration manually**, especially the first and last lines, as these serve as markers so kube-botblocker can track where its configuration starts/end, keeping the rest of your configuration inside the annotation intact. If you do remove or edit the markers, the contents of the `server-snippet` annotation will be preserved, but manual action will be required to clean up any leftover configuration added by the operator.

Updating the IngressConfig object (adding or removing user agents) will roll out an update to the `server-snippet` annotation of all Ingresses that reference said IngressConfig.

When you want to remove the generated configuration, remove the `kube-botblocker.github.io/ingressConfigName` annotation manually or using the command bellow:

```bash
kubectl annotate ingress -n your-namespace your-ingress kube-botblocker.github.io/ingressConfigName-
```

kube-botblocker will then remove the generated configuration, leaving the pre-existing configuration (if any) intact.

Mass addition/removal of annotations can be achieved using `-A` and `--all` flags for `kubectl annotate`:

```bash
# Add annotations for all Ingresses in a certain namespace
kubectl annotate ingress -n your-namespace --all kube-botblocker.github.io/ingressConfigName=useragents-blocklist

# Add annotations for all ingress in all namespaces
kubectl annotate ingress -A --all kube-botblocker.github.io/ingressConfigName=useragents-blocklist

# Remove annotations for all Ingresses in a certain namespace
kubectl annotate ingress -n your-namespace --all kube-botblocker.github.io/ingressConfigName-

# Remove annotations for all ingress in all namespaces
kubectl annotate ingress -A --all kube-botblocker.github.io/ingressConfigName-
```

### Deployment modes
kube-botblocker has two deployment modes that can be toggled using the `currentNamespaceOnly` parameter present in the chart `values.yaml`:

`currentNamespaceOnly: false` - Default, allows kube-botblocker annotations work on all Ingresses cluster wide.

`currentNamespaceOnly: true` - Restricts kube-botblocker annotations to work only on Ingresses present in the same namespace as itself. Annotations applied to Ingresses in other namespaces **will be ignored**.
