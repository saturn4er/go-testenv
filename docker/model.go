package docker

type RunContainerNetworkConfig struct {
	Aliases []string
}
type PortBinding struct {
	Host string
	Port string
}

type RunContainerParams struct {
	ContainerName string
	Envs          map[string]string
	Image         string
	ExposedPorts  []string
	Cmd           []string
	Networks      map[string]RunContainerNetworkConfig
	Labels        map[string]string
	PortBindings  map[string][]PortBinding
}

type CreateNetworkParams struct {
	Driver  string
	Options map[string]interface{}
	Labels  map[string]string
}

type BuildImageParams struct {
	Dockerfile string
	ContextDir string
	Labels     map[string]string
	BuildArgs  map[string]string
}
