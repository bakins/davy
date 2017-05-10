# davy

A simple utility for creating Kubernetes manifests from templates and metadata.

## Status

This is really more of an idea sketched out in code. This README represents what
I envision in such a tool and not necessarily what it does currently.

It's named for [Davy Jones](https://en.wikipedia.org/wiki/Davy_Jones%27_Locker)
as container related projects always seem to have nautical themes.

## Philosophy/Approach

The approach is that generating Kubernetes manifests is completely separate from
deploying them. Davy's job is to generate Kubernetes manifests. It has no state
and no server. Davy could be completely rewritten or replaced, and the 
generated manifests could be managed by a different tool. The inputs for Davy
are just files on disk, but this could be wrapped in CI/CD tooling.

While this is not as "shiny" as some interesting API in front of Kubernetes, it
is farily simple and the Kubernetes manifests are all in git. You can use familiar
git tooling to track changes, roll forward and back.

Influenced by:
- [Kubernetes at Box](https://blog.box.com/blog/kubernetes-box-microservices-maximum-velocity/)
- [Helm](https://github.com/kubernetes/helm)

## Workflows

Some example workflows:

Pure git workflow:
1. an engineer edits a template and/or metadata
2. the engineer runs davy to generate the manifests
3. the templates and the manifests are committed to a branch in a repository
4. A PR is opened. One can easily look at the PR and see exactly what Kubernetes
resources will be changed.  This avoids a simple metadata change from
changing dozens of resources unexpectedly.
5. When the PR is merged to master, a simple wrapper around `kubectl apply`
deploys the resources. As `kubectl apply` should be idempotent, it can apply 
everything.

Jenkins (or other CI tooling) could be involved in the workflow:
- Jenkins could run davy. This could be committed to the branch or simply
presented to the engineer.
- Jenkins could run a wrapper around `kubectl apply --dry-run` and show the 
changes that would be made.
- Jenkins could run `kubectl apply`

davy keeps track of what manifests change on a given run, so one could just apply 
those. One could also use git to determine what manifests changed in a PR.

## Directory Layout

davy has input files and output files.  

Output files are arranged like: `./<cluster>/<namespace>/<app>/<resource>.yaml`
Where an application can have multiple resources. One can deploy this to a cluster
with a simple wrapper that basically does 
`kubectl apply -r --context=<cluster -f ./<cluster>` - though you would need to 
ensure the namespaces were created.

There are three types of input files: resource templates, helpers, and metadata.

Resource templates are templated (using 
[go text/template](https://golang.org/pkg/text/template/)) 
YAML files for Kubernetes resources. One resource per file.  The template should
generate a complete, valid Kubernetes resource.  These templates are in an a directory
per application. The name of the directory is the name of the application.  You
can share templates between applications by using simple symlinks. One could also
use symlinks for common components.

Helpers are [templates](https://golang.org/pkg/text/template/#example_Template_helpers)
that can be used to encapsulate common patterns. All helpers are in a single
directory. 

Metadata is used by davy to render the templates. Config metadata lives in
the same directory as the application templates in YAML files that begin with
an underscore ("\_").  The form of these YAML files are:

```yaml
name: name of the config - defaults to name of file _<name>.yaml
namespace: Kubernetes Namespace to deploy into.
clusters: array of clusters to deploy to.
env: environment ("prod", "dev", "stage", etc)
values: key value pairs of arbitrary data.
```
At least one cluster is required.

The  cluster names refer to metadata files in the cluster directory.  
A file with the cluster name and a .yaml extension is expected to be found. The
form of the cluster metadata files are:

```yaml
values: key value pairs of arbitrary data.
```
The env refers to a metdata file in the env directory.  It has the same format 
as the cluster metadata files.

The values are merged with the following precedence (highest to lowest):
- app config
- cluster metadata
- env metadata

(nested merging TBD)

The following data structure is presented to the template:

* AppName - name of the application
* ConfigName - name of the config used
* Namespace
* Cluster  
* Env
* Values

So, AppName can be referenced in the template as `{{ .AppName }}`. A key "foo" from
the metadata files can be referenced as `{{ .Values.foo }}`


## TODO
- what about Kubernetes federation?
- do we need secrets integration?
- image pull secrets, resource limits per namespace, etc
- track items that may need deleting? or should an auditing tool track these 
separately.
- switch to [pongo2](https://github.com/flosch/pongo2)?
- per app helper templates?


## LICENSE
see [LICENSE](./LICENSE)
