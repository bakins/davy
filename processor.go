package davy

import (
	"bytes"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

// Processor will process a single config and template
type Processor struct {
	configs []*Config
	g       *Generator
	name    string
}

type templateContext struct {
	AppName    string
	ConfigName string
	Namespace  string
	Cluster    string
	Env        string
	Values     Values
}

// NewProcessor will create a new processor. It will merge the
// config with the cluster and env configs.
// Name is the name of the application
func (g *Generator) NewProcessor(name string, c *Config) (*Processor, error) {
	if len(c.Clusters) == 0 {
		return nil, errors.New("no clusters defined")
	}

	if c.Namespace == "" {
		return nil, errors.New("no namespace defined")
	}

	// TODO: cache env and clusters configs on generator?

	p := &Processor{
		name: name,
		g:    g,
	}

	// we create a new config for each cluster
	// merge precedence for values is env, cluster, passed in config
	for _, cluster := range c.Clusters {
		out := &Config{}
		*out = *c

		if c.Env != "" {
			envConfig, err := g.loadEnvConfig(c.Env)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to load env %s", c.Env)
			}
			out.Values = MergeValues(envConfig.Values, out.Values)
		}

		clusterConfig, err := g.loadClusterConfig(cluster)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to load cluster %s", cluster)
		}

		out.Values = MergeValues(clusterConfig.Values, out.Values)

		// just set clusters to this single cluster
		out.Clusters = []string{cluster}

		p.configs = append(p.configs, out)
	}

	return p, nil
}

// ProcessTemplate will process a single template.
func (p *Processor) ProcessTemplate(filename string) (map[string][]byte, error) {
	output := make(map[string][]byte)

	for _, c := range p.configs {
		t, err := p.g.helpers.Clone()
		if err != nil {
			return nil, errors.Wrap(err, "failed to clone helpers")
		}

		tmpl, err := t.ParseFiles(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse template: %f", filename)
		}

		ctx := templateContext{
			AppName:    p.name,
			ConfigName: c.Name,
			Namespace:  c.Namespace,
			Cluster:    c.Clusters[0],
			Env:        c.Env,
			Values:     c.Values,
		}

		var buf bytes.Buffer
		if err := tmpl.Option("missingkey=error").ExecuteTemplate(&buf, filepath.Base(filename), ctx); err != nil {
			return nil, errors.Wrapf(err, "failed to execute template: %s", filename)
		}

		data := buf.Bytes()

		if err := p.validateOutput(data); err != nil {
			return nil, errors.Wrapf(err, "failed to validate output for %s", filename)
		}

		outFile := filepath.Join(c.Clusters[0], c.Namespace, p.name, filepath.Base(filename))
		output[outFile] = data
	}

	return output, nil
}

// Object is a Kubernetes object
type Object struct {
	Kind       string   `json:"kind"`
	APIVersion string   `json:"apiVersion"`
	Metadata   Metadata `json:"metadata"`
}

// Metadata is Kubernetes metadata
type Metadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func (p *Processor) validateOutput(data []byte) error {
	var o Object
	if err := yaml.Unmarshal(data, &o); err != nil {
		return errors.Wrap(err, "unable to unmarshal data")
	}

	if o.Kind == "" {
		return errors.New("kind must be set")
	}

	if o.APIVersion == "" {
		return errors.New("apiVersion must be set")
	}

	if o.Metadata.Name == "" {
		return errors.New("metadata.name must be set")
	}

	return nil
}
