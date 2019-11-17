package docker

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/ory/dockertest/docker"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/multierr"
)

type Client struct {
	client *docker.Client

	createdContainers map[string]*docker.Container
	createdNetworks   map[string]*docker.Network
	builtImages       map[string]struct{}
}

func (c *Client) Cleanup() error {
	var err error
	for id := range c.createdContainers {
		if removeErr := c.RemoveContainer(id); removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}

	for id := range c.createdNetworks {
		if removeErr := c.RemoveNetwork(id); removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}
	for imageName := range c.builtImages {
		if removeErr := c.RemoveImage(imageName); removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}

	return err
}

func (c *Client) RemoveContainer(id string) error {
	err := c.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            id,
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	delete(c.createdContainers, id)
	return nil
}

func (c *Client) RemoveNetwork(id string) error {
	err := c.client.RemoveNetwork(id)
	if err != nil {
		return errors.WithStack(err)
	}

	delete(c.createdNetworks, id)
	return nil
}

func (c *Client) RemoveImage(id string) error {
	err := c.client.RemoveImageExtended(id, docker.RemoveImageOptions{
		Force: true,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	delete(c.builtImages, id)

	return nil
}
func (c *Client) CreateNetwork(params CreateNetworkParams) (*docker.Network, error) {
	network, err := c.client.CreateNetwork(docker.CreateNetworkOptions{
		Name:    uuid.NewV4().String(),
		Driver:  params.Driver,
		Options: params.Options,
		Labels:  params.Labels,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c.createdNetworks[network.ID] = network
	return network, nil
}

func (c *Client) RunContainer(params RunContainerParams) (*docker.Container, error) {
	container, err := c.createContainer(params)
	if err != nil {
		if errors.Cause(err) != docker.ErrNoSuchImage {
			return nil, errors.WithStack(err)
		}

		if err := c.PullImage(params.Image); err != nil {
			return nil, errors.WithStack(err)
		}
		container, err = c.createContainer(params)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return container, nil
}

func (c *Client) PullImage(image string) error {
	authConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.WithStack(err)
		}
	}

	var authConfig docker.AuthConfiguration
	for server, config := range authConfigs.Configs {
		if strings.HasPrefix(image, server) {
			authConfig = config
			break
		}
	}

	imageParts := strings.Split(image, ":")
	err = c.client.PullImage(docker.PullImageOptions{
		Repository: imageParts[0],
		Tag:        imageParts[1],
	}, authConfig)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) BuildImage(params BuildImageParams) (string, error) {
	authConfigs, err := c.dockerAuthConfigs()
	if err != nil {
		return "", errors.WithStack(err)
	}

	buildArgs := make([]docker.BuildArg, 0, len(params.BuildArgs))
	for key, value := range params.BuildArgs {
		buildArgs = append(buildArgs, docker.BuildArg{
			Name:  key,
			Value: value,
		})
	}

	imageName := uuid.NewV4().String()
	err = c.client.BuildImage(docker.BuildImageOptions{
		Name:         imageName,
		Dockerfile:   params.Dockerfile,
		ContextDir:   params.ContextDir,
		Labels:       params.Labels,
		OutputStream: ioutil.Discard,
		AuthConfigs:  *authConfigs,
		BuildArgs:    buildArgs,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	c.builtImages[imageName] = struct{}{}

	return imageName, nil
}

func (c *Client) createContainer(params RunContainerParams) (*docker.Container, error) {
	exposedPorts := make(map[docker.Port]struct{})
	for _, exposedPort := range params.ExposedPorts {
		exposedPorts[docker.Port(exposedPort)] = struct{}{}
	}

	envs := make([]string, 0, len(params.Envs))
	for key, value := range params.Envs {
		envs = append(envs, key+"="+value)
	}

	portBindings := make(map[docker.Port][]docker.PortBinding)
	for port, bindings := range params.PortBindings {
		dockerBindings := make([]docker.PortBinding, 0, len(bindings))
		for _, binding := range bindings {
			dockerBindings = append(dockerBindings, docker.PortBinding{
				HostIP:   binding.Host,
				HostPort: binding.Port,
			})
		}
		portBindings[docker.Port(port)] = dockerBindings
	}

	container, err := c.client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd:          params.Cmd,
			Env:          envs,
			Image:        params.Image,
			ExposedPorts: exposedPorts,
			StopSignal:   "SIGWINCH", // to support timeouts
			Labels:       params.Labels,
		},
		HostConfig: &docker.HostConfig{
			PublishAllPorts: true,
			PortBindings:    portBindings,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.createdContainers[container.ID] = container

	if err := c.client.StartContainer(container.ID, nil); err != nil {
		return nil, errors.WithStack(err)
	}

	for networkID, networkConfig := range params.Networks {
		err := c.client.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
			Container: container.ID,
			EndpointConfig: &docker.EndpointConfig{
				Aliases: networkConfig.Aliases,
			},
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	container, err = c.client.InspectContainer(container.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return container, err
}

func (c *Client) dockerAuthConfigs() (*docker.AuthConfigurations, error) {
	cfg, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		if os.IsNotExist(err) {
			return &docker.AuthConfigurations{}, nil
		}
		return nil, errors.Wrap(err, "failed to resolve docker auth configurations")
	}

	return cfg, nil
}

func NewClient() (*Client, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &Client{
		client:            client,
		createdContainers: map[string]*docker.Container{},
		createdNetworks:   map[string]*docker.Network{},
		builtImages:       map[string]struct{}{},
	}, nil
}
