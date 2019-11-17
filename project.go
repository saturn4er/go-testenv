package testenv

import (
	"log"
	"sync"

	"github.com/pkg/errors"
	"github.com/saturn4er/go-testenv/docker"
)

type ProjectEnvDesc struct {
	Networks    map[string]NetworkDesc
	Containers  map[string]ContainerDesc
	TestCaseEnv TestCaseEnvDesc
}

type ProjectEnv struct {
	client *docker.Client
	desc   ProjectEnvDesc

	createdNetworks   map[string]*Network
	builtImages       map[string]string
	createdContainers map[string]*Container

	variablesMx sync.RWMutex
	variables   map[string]interface{}
}

func (p *ProjectEnv) Run() error {
	if err := p.createNetworks(); err != nil {
		return errors.WithStack(err)
	}

	if err := p.runContainers(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *ProjectEnv) Close() error {
	return errors.WithStack(p.client.Cleanup())
}

func (p *ProjectEnv) NewTestCaseEnv() *TestCaseEnv {
	return &TestCaseEnv{
		projectEnv:        p,
		createdNetworks:   map[string]*Network{},
		createdContainers: map[string]*Container{},
	}
}

func (p *ProjectEnv) Container(name string) (*Container, bool) {
	container, ok := p.createdContainers[name]
	return container, ok
}

func (p *ProjectEnv) FindFreeHostPort() (string, error) {
	container, err := ContainerDesc{
		Image:        ExternalImage("alpine:latest"),
		ExposedPorts: []StringValueResolver{StringValue("9999")},
	}.run(p, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to run temporary container")
	}

	defer func() {
		if err := p.client.RemoveContainer(container.ID); err != nil {
			log.Printf("Failed to remove temorary container(ID: %s)", container.ID)
		}
	}()

	port, ok := (&Container{container: container}).HostPort("9999", PortTypeTCP)
	if !ok {
		return "", errors.New("internal error")
	}

	return port, nil
}

func (p *ProjectEnv) Set(key string, value interface{}) {
	p.variablesMx.Lock()
	p.variables[key] = value
	p.variablesMx.Unlock()
}

func (p *ProjectEnv) Get(key string) interface{} {
	p.variablesMx.RLock()
	defer p.variablesMx.RUnlock()

	return p.variables[key]
}

func (p *ProjectEnv) createNetworks() error {
	for networkName, networkDesc := range p.desc.Networks {
		log.Printf("Creating project network %s", networkName)
		network, err := networkDesc.create(p, nil)
		if err != nil {
			return err
		}
		log.Printf("Created network %s (ID: %s)", networkName, network.ID)

		p.createdNetworks[networkName] = &Network{
			ID:         network.ID,
			Name:       networkName,
			DockerName: network.Name,
		}
	}
	return nil
}
func (p *ProjectEnv) runContainers() error {
	for containerName, containerDesc := range p.desc.Containers {
		log.Printf("Creating project container %s", containerName)
		container, err := containerDesc.run(p, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to run container %s", containerName)
		}

		log.Printf("Created container %s (ID: %s)", containerName, container.ID)
		p.createdContainers[containerName] = &Container{container: container}
	}

	return nil
}

func NewProjectEnv(desc ProjectEnvDesc) (*ProjectEnv, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &ProjectEnv{
		client:            dockerClient,
		desc:              desc,
		variables:         map[string]interface{}{},
		createdNetworks:   map[string]*Network{},
		builtImages:       map[string]string{},
		createdContainers: map[string]*Container{},
	}, nil

}
