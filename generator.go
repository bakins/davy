package davy

import (
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/davecgh/go-spew/spew"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

// OptionsFunc is a function passed to new for setting options on a new
// generator.
type OptionsFunc func(*Generator) error

// Generator handles reading templates and generating output
type Generator struct {
	// directory that has cluster overlays
	clusterDir string
	// directory that has env overlays
	envDir string

	// where to write output
	outDir  string
	helpers *template.Template
}

func (g *Generator) doOptions(options []OptionsFunc) error {
	for _, f := range options {
		if err := f(g); err != nil {
			return errors.Wrap(err, "options function failed")
		}
	}
	return nil
}

// New creates a Generator.
func New(options ...OptionsFunc) (*Generator, error) {
	g := &Generator{
		helpers: template.New("base").Option("missingkey=error").Funcs(sprig.TxtFuncMap()),
	}

	if err := g.doOptions(options); err != nil {
		return nil, err
	}

	return g, nil
}

func testDir(name string) error {
	info, err := os.Stat(name)
	if err != nil {
		return errors.Wrap(err, "failed to stat directory")
	}
	if !info.IsDir() {
		return errors.Errorf("%s is not a directory", name)
	}
	return nil
}

// SetClusterDir creates a function that will set the cluster overlay directory.
// Generally, only used when create a new Generator.
func SetClusterDir(name string) func(*Generator) error {
	return func(g *Generator) error {
		if err := testDir(name); err != nil {
			return err
		}
		g.clusterDir = name
		return nil
	}
}

// SetOutDir creates a function that will set the output directory.
// Generally, only used when create a new Generator.
func SetOutDir(name string) func(*Generator) error {
	return func(g *Generator) error {
		// we will attempt to create this later
		g.outDir = name
		return nil
	}
}

// SetEnvDir creates a function that will set the env overlay directory.
// Generally, only used when create a new Generator.
func SetEnvDir(name string) func(*Generator) error {
	return func(g *Generator) error {
		if err := testDir(name); err != nil {
			return err
		}
		g.envDir = name
		return nil
	}
}

// ConfigFromBytes reads a config from bytes of yaml.
func (g *Generator) ConfigFromBytes(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}
	return &c, nil
}

// ConfigFromFile reads a config from a yaml file.
func (g *Generator) ConfigFromFile(file string) (*Config, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}
	c, err := g.ConfigFromBytes(data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse config %s", file)
	}

	if c.Name == "" {
		// name defaults to filename with _ removed and without extension
		c.Name = strings.TrimSuffix(
			strings.TrimPrefix(filepath.Base(file), "_"),
			".yaml",
		)
	}
	return c, nil
}

// ReadHelpers will read helper templates from the given glob.
// This will add to any existing helpers already loaded
func (g *Generator) ReadHelpers(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return errors.Wrap(err, "failed to get list of files")
	}

	t, err := g.helpers.Clone()
	if err != nil {
		return errors.Wrap(err, "failed to clone helpers")
	}
	for _, filename := range files {
		t, err = t.ParseFiles(filename)
		if err != nil {
			return errors.Wrapf(err, "failed to parse file: %s", filename)
		}
	}

	g.helpers = t

	return nil
}

// TODO: function to merge configs

// ProcessDir will process a single directory and write it to output dir.
// configs are assumed to match _*.yaml. templates are all other yaml files.
func (g *Generator) ProcessDir(in string) error {
	dir, err := filepath.Abs(in)
	if err != nil {
		return errors.Wrapf(err, "failed to get absolute path of %s", in)
	}

	appName := filepath.Base(dir)

	files, err := filepath.Glob(filepath.Join(in, "*.yaml"))
	if err != nil {
		return errors.Wrap(err, "failed to get list of files")
	}

	var templates []string
	configMap := make(map[string]*Config)
	var processors []*Processor
	for _, filename := range files {
		if strings.HasPrefix(filepath.Base(filename), "_") {
			c, err := g.ConfigFromFile(filename)
			if err != nil {
				return errors.Wrap(err, "failed to load config")
			}

			if _, ok := configMap[c.Name]; ok {
				return errors.Errorf("a config named %s already exists", c.Name)
			}
			configMap[c.Name] = c

			p, err := g.NewProcessor(appName, c)
			if err != nil {
				return errors.Wrap(err, "failed to create processor")
			}
			processors = append(processors, p)
		} else {
			templates = append(templates, filename)
		}
	}

	// processes each template with each processor
	// TODO: cache templates ?
	for _, p := range processors {
		for _, t := range templates {
			outFiles, err := p.ProcessTemplate(t)
			if err != nil {
				return errors.Wrapf(err, "failed to process %s", t)
			}

			for k, v := range outFiles {
				if err := g.writeDataFile(k, v); err != nil {
					return errors.Wrapf(err, "failed to process %s => %s", t, v)
				}
			}
		}

	}

	return nil
}

func (g *Generator) loadClusterConfig(name string) (*Config, error) {
	return g.ConfigFromFile(filepath.Join(g.clusterDir, name+".yaml"))
}

func (g *Generator) loadEnvConfig(name string) (*Config, error) {
	return g.ConfigFromFile(filepath.Join(g.envDir, name+".yaml"))
}

func (g *Generator) writeDataFile(name string, data []byte) error {
	outfile := filepath.Join(g.outDir, name)

	// only write if needed
	if !shouldWrite(data, outfile) {
		return nil
	}

	outDir := filepath.Dir(outfile)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}
	tmp, err := ioutil.TempFile(outDir, ".tmp.")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}

	// if we sucessfully rename the file, this will error, which is fine
	defer os.Remove(tmp.Name())

	_, err = tmp.Write(data)
	if err != nil {
		return errors.Wrap(err, "failed to write to temp file")
	}

	if err = os.Rename(tmp.Name(), outfile); err != nil {
		return errors.Wrap(err, "failed to rename file")
	}

	return nil
}

func shouldWrite(data []byte, filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		// if it's a permission problem, we will handle later
		return true
	}

	fileData, err := ioutil.ReadFile(filename)
	if err != nil {
		return true
	}

	var fileMap map[string]interface{}
	if err = yaml.Unmarshal(fileData, &fileMap); err != nil {
		return true
	}

	var dataMap map[string]interface{}
	if err = yaml.Unmarshal(data, &dataMap); err != nil {
		// should this ever happen?
		return true
	}

	a := hashMap(fileMap)
	b := hashMap(dataMap)

	return a != b
}

func hashMap(in map[string]interface{}) uint64 {
	// from k8s
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}

	hasher := fnv.New64()
	printer.Fprintf(hasher, "%#v", in)

	return hasher.Sum64()
}
