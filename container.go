package testenv

import (
	dc "github.com/ory/dockertest/docker"
	"github.com/pkg/errors"
	"github.com/saturn4er/go-testenv/docker"
)

type HealthCheck func() (bool, error)

type ContainerHooks struct {
	BeforeRun func(project *ProjectEnv, testCase *TestCaseEnv) error
	AfterRun  func(project *ProjectEnv, testCase *TestCaseEnv) error
}

type PortBinding struct {
	Host          StringValueResolver
	Port          StringValueResolver
	ContainerPort StringValueResolver
}

type ContainerDesc struct {
	Image        ImageResolver
	HealthCheck  *HealthCheck
	Envs         map[string]StringValueResolver
	ExposedPorts []StringValueResolver
	Networks     []ContainerNetwork
	Labels       map[string]StringValueResolver
	Cmd          []string
	Hooks        ContainerHooks
	PortBindings []PortBinding
}

func (c Container) HostPort(port string, portType PortType) (string, bool) {
	if c.container == nil {
		return "", false
	} else if c.container.NetworkSettings == nil {
		return "", false
	}

	m, ok := c.container.NetworkSettings.Ports[dc.Port(port+"/"+portType.String())]
	if !ok {
		return "", false
	} else if len(m) == 0 {
		return "", false
	}

	return m[0].HostPort, true
}

func (c ContainerDesc) run(project *ProjectEnv, testCase *TestCaseEnv) (*dc.Container, error) {
	if c.Hooks.BeforeRun != nil {
		if err := c.Hooks.BeforeRun(project, testCase); err != nil {
			return nil, errors.Wrap(err, "failed to process 'BeforeRun' hook")
		}
	}
	image, err := c.Image(project, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve image")
	}

	networks := map[string]docker.RunContainerNetworkConfig{}
	for _, containerNetwork := range c.Networks {
		network, err := containerNetwork.Network(project, testCase)
		if err != nil {
			return nil, errors.Wrap(err, "failed to resolve network")
		}

		cfg := docker.RunContainerNetworkConfig{}
		if containerNetwork.Alias != "" {
			cfg.Aliases = append(cfg.Aliases, containerNetwork.Alias)
		}

		networks[network] = cfg
	}

	labels := make(map[string]string)
	for labelName, labelValueResolver := range c.Labels {
		value, err := labelValueResolver(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve label %s value", labelName)
		}
		labels[labelName] = value
	}

	envs := make(map[string]string)
	for envName, envValueResolver := range c.Envs {
		value, err := envValueResolver(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve env %s value", envName)
		}
		envs[envName] = value
	}

	portBindings := make(map[string][]docker.PortBinding)
	for i, binding := range c.PortBindings {
		host, err := binding.Host(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve port binding #%d host", i)
		}

		port, err := binding.Port(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve port binding #%d port", i)
		}

		containerPort, err := binding.ContainerPort(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve port binding #%d container port", i)
		}

		portBindings[containerPort] = append(portBindings[containerPort], docker.PortBinding{
			Host: host,
			Port: port,
		})
	}

	exposedPorts := make([]string, 0, len(c.ExposedPorts))
	for i, exposedPortResolver := range c.ExposedPorts {
		exposedPort, err := exposedPortResolver(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve %d exposed port", i)
		}
		exposedPorts = append(exposedPorts, exposedPort)
	}

	container, err := project.client.RunContainer(docker.RunContainerParams{
		Envs:         envs,
		Image:        image,
		ExposedPorts: exposedPorts,
		Cmd:          c.Cmd,
		Networks:     networks,
		Labels:       labels,
		PortBindings: portBindings,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if c.Hooks.AfterRun != nil {
		if err := c.Hooks.AfterRun(project, testCase); err != nil {
			return nil, errors.Wrap(err, "failed to process 'AfterRun' hook")
		}
	}
	return container, nil
}

type ContainerNetwork struct {
	Network NetworkResolver
	Alias   string
}

type Container struct {
	container *dc.Container
}
