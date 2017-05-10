package davy

// Values is a map of key/value pairs
type Values map[string]interface{}

// Config is a single application config
type Config struct {
	// name of the application. defaults to directory name the config is found in.
	Name      string   `json:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
	Env       string   `json:"env,omitempty"` // env: prod, dev, stage, etc
	Values    Values   `json:"values"`
}

// MergeValues will merge src over a copy of dest and return this new map.
// This is currently very dumb, TODO make it traverse maps, etc
func MergeValues(dst, src Values) Values {
	out := make(Values, len(src))
	for k, v := range dst {
		out[k] = v
	}

	for k, v := range src {
		out[k] = v
	}

	return out
}
