# Service-discovery

# Pipeline

Service-discovery pipeline has 4 jobs:

-   [discovery](#Discovery)
-   [tag](#Tag)
-   [build](#Build)
-   [export](#Export)


Configuration:

```yaml
discovery: <discovery_config>
tag: <tag_config>
build: <build_config>
export: <export_config>
```

# Discovery

Discovery job dynamically discovers targets using one of the supported service-discovery mechanisms.

Supported mechanisms:

-   `kubernetes`

Configuration:

```yaml
k8s:
  - <kubernetes_discovery_config>
```

## Kubernetes

Kubernetes discoverer retrieves targets from [Kubernetes'](https://kubernetes.io/) [REST API](https://kubernetes.io/docs/reference/).
It always stays synchronized with the cluster state.

One of the following role types can be configured to discover targets:

-   `pod`
-   `service`

### Pod Role

The pod role discovers all pods and exposes their containers as targets.
For each declared port of a container, it generates single target.

Available pod target fields:

-   `TUID`: equal to `Namespace_Name_ContName_PortProtocol_Port`.
-   `Address`: equal to `PodIP:Port`.
-   `Namespace`: is _pod.metadata.namespace_.
-   `Name`: is _pod.metadata.name_.
-   `Annotations`: is a dict that contains all annotations from _pod.metadata.annotations_.
-   `Labels`: is a dict that contains all labels from _pod.metadata.labels_.
-   `NodeName`: is _pod.spec.nodeName_. 
-   `PodIP`: is _pod.status.podIP_. 
-   `ContName`: is _pod.spec.containers.name_. 
-   `Image`: is _pod.spec.containers.image_.
-   `Env`: is a dict that contains all variables from _pod.spec.containers.env_ and _pod.spec.containers.envFrom_.
-   `Port`: is _pod.spec.containers.ports.containerPort_.
-   `PortName`: is _pod.spec.containers.ports.name_.
-   `PortProtocol`: is _pod.spec.containers.ports.protocol_.

### Service Role

The service role discovers a target for each service port for each service.

Available service target fields:

-   `TUID`: equal to `Namespace_Name_PortProtocol_Port`.
-   `Address`: equal to `Name.Namespace.svc:Port`.
-   `Namespace`: is _svc.metadata.namespace_.
-   `Name`: is _svc.metadata.name_.
-   `Annotations`: is a dict that contains all annotations from _svc.metadata.annotations_.
-   `Labels`: is a dict that contains all labels from _svc.metadata.labels_.
-   `Port`: is _svc.spec.ports.port_.
-   `PortName`: is _svc.spec.ports.name_.
-   `PortProtocol`: is _svc.spec.ports.protocol_.
-   `ClusterIP`: is _svc.spec.clusterIP_.
-   `ExternalName`: is _svc.spec.externalName_.
-   `Type`: is _svc.spec.ports.type_.

### Configuration

Kubernetes discovery configuration options:

```yaml
# Mandatory. List of tags to add for all discovered targets.
tags: <tags>

# Mandatory. The Kubernetes role of entities that should be discovered.
role: <role>

# Optional. Discover only targets that exist on the same node as service-discovery.
# This option works only for 'pod' role and it requires MY_NODE_NAME env variable to be set.
local_mode: <boolean>

# Optional. If omitted, all namespaces are used.
namespaces:
  - <namespace>
```

# Tag

Tag job tags targets discovered by [discovery job](#Discovery) based on defined conditions.

## Configuration

Configuration is a list of tag rules:

```yaml
- <tag_rule_config>
```

Tag rule configuration options:

```yaml
# Mandatory. List of selectors to check against target tags.
selector: <selector>

# Mandatory. List of tags to merge with the target tags if at least on of the match rules matches.
tags: <tags>

# Mandatory. List of match rules. At least one should be defined. 
match:
      # Mandatory. List of tags to merge with the target tags if this rule condition matches the target.
    - tags: <tags>

      # Optional. List of selectors to check against target tags.
      selector: <selector>

      # Mandatory. Match expression.
      expr: <expression>
```

**Match expression evaluation result should be true/false**.
Expression syntax is [go-template](https://golang.org/pkg/text/template/).

In addition, the following functions are available:

-   `glob(value, patterns...) bool`   
-   `regexp(value, patterns...) bool` 
-   `equal(value, values...) bool` 
-   `hasKey(value, keys...) bool`

`glob` uses [gobwas/glob](https://github.com/gobwas/glob) library, see [pattern syntax](https://github.com/gobwas/glob/blob/f00a7392b43971b2fdb562418faab1f18da2067a/glob.go#L13-L38).

# Build

Build job creates configurations from targets based on defined templates.

## Configuration

Configuration is a list of build rules:

```yaml
- <build_rule_config>
```

Build rule configuration options:

```yaml
# Mandatory. List of selectors to check against target tags.
selector: <selector>

# Mandatory. List of tags to add to all built configurations.
tags: <tags>

# Mandatory. List of apply rules. At least one should be defined. 
apply:
      # Optional. List of tags to add to the configuration built on this step.
    - tags: <tags>

      # Mandatory. List of selectors to check against target tags.
      selector: <selector>

      # Mandatory. Configuration template.
      template: <template>
```

Template syntax is [go-template](https://golang.org/pkg/text/template/).

# Export

Export job exports configurations built by [build job](#Build) based on defined destinations.

Supported exporters:

-   `file`

## Configuration

```yaml
file:
  - <file_exporter_config>
```

## File

```yaml
# Mandatory. List of selectors to check against configuration tags.
selector: <selector>
# Mandatory. Absolute path to a file.
filename: <filename>
```

# CLI

```cmd
Usage:
  sd [OPTION]...

Application Options:
      --config-file= Configuration file path
      --config-map=  Configuration ConfigMap (name:key)
  -d, --debug        Debug mode

Help Options:
  -h, --help         Show this help message
```
