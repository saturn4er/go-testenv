package testenv

import (
	dc "github.com/ory/dockertest/docker"
	"github.com/pkg/errors"
	"github.com/saturn4er/go-testenv/docker"
)

type NetworkResolver func(project *ProjectEnv, testCase *TestCaseEnv) (string, error)

func ProjectNetwork(name string) NetworkResolver {
	return func(project *ProjectEnv, testCase *TestCaseEnv) (string, error) {
		if project == nil {
			return "", errors.New("no project passed")
		}
		network, ok := project.createdNetworks[name]
		if !ok {
			return "", errors.Errorf("no network %s in project", name)
		}
		return network.ID, nil
	}
}

func TestCaseNetwork(name string) NetworkResolver {
	return func(project *ProjectEnv, testCase *TestCaseEnv) (string, error) {
		if testCase == nil {
			return "", errors.New("can't use TestCaseNetwork resolver in project scope")
		}

		network, ok := testCase.createdNetworks[name]
		if !ok {
			return "", errors.Errorf("no network %s in test case", name)
		}
		return network.ID, nil
	}
}

type NetworkDesc struct {
	Labels StringsMap
}

func (n *NetworkDesc) create(project *ProjectEnv, testCase *TestCaseEnv) (*dc.Network, error) {
	labels, err := n.Labels.resolve(project, testCase)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	network, err := project.client.CreateNetwork(docker.CreateNetworkParams{
		Labels: labels,
	})
	if err != nil {
		return nil, err
	}

	return network, nil
}

type Network struct {
	ID         string
	Name       string
	DockerName string
}
