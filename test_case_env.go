package testenv

import (
	"log"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type PortType byte

func (p PortType) String() string {
	switch p {
	case PortTypeTCP:
		return "tcp"
	case PortTypeUDP:
		return "udb"
	}
	return "unknown"
}

const (
	PortTypeTCP PortType = 1 + iota
	PortTypeUDP
)

type TestCaseEnv struct {
	projectEnv        *ProjectEnv
	createdNetworks   map[string]*Network
	createdContainers map[string]*Container
}

func (t *TestCaseEnv) Run() error {
	if err := t.createNetworks(); err != nil {
		return errors.WithStack(err)
	}

	if err := t.runContainers(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (t *TestCaseEnv) Close() error {
	var err error

	for _, container := range t.createdContainers {
		if removeErr := t.projectEnv.client.RemoveContainer(container.container.ID); removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}

	for _, network := range t.createdNetworks {
		if removeErr := t.projectEnv.client.RemoveNetwork(network.ID); removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}

	return nil
}

func (p *TestCaseEnv) Container(name string) (*Container, bool) {
	container, ok := p.createdContainers[name]
	return container, ok
}

func (t *TestCaseEnv) createNetworks() error {
	for networkName, networkDesc := range t.projectEnv.desc.TestCaseEnv.Networks {
		log.Printf("Creating project network %s", networkName)
		network, err := networkDesc.create(t.projectEnv, t)
		if err != nil {
			return err
		}
		log.Printf("Created network %s (ID: %s)", networkName, network.ID)

		t.createdNetworks[networkName] = &Network{
			ID:         network.ID,
			Name:       networkName,
			DockerName: network.Name,
		}
	}
	return nil
}
func (t *TestCaseEnv) runContainers() error {
	for containerName, containerDesc := range t.projectEnv.desc.TestCaseEnv.Containers {
		log.Printf("Creating project container %s", containerName)
		container, err := containerDesc.run(t.projectEnv, t)
		if err != nil {
			return errors.Wrapf(err, "failed to run container %s", containerName)
		}

		log.Printf("Created container %s (ID: %s)", containerName, container.ID)
		t.createdContainers[containerName] = &Container{container: container}
	}

	return nil
}

type TestCaseEnvDesc struct {
	Networks   map[string]NetworkDesc
	Containers map[string]*ContainerDesc
}
