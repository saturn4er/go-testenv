package testenv

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/saturn4er/go-testenv/docker"
)

type BuildImageParams struct {
}

type ImageResolver func(project *ProjectEnv, caseEnv *TestCaseEnv) (string, error)

func ExternalImage(image string) ImageResolver {
	return func(project *ProjectEnv, caseEnv *TestCaseEnv) (string, error) {
		return image, nil
	}
}

func BuildImage(desc ImageDesc) ImageResolver {
	var image string
	var err error

	once := &sync.Once{}
	return func(project *ProjectEnv, caseEnv *TestCaseEnv) (string, error) {
		once.Do(func() {
			image, err = desc.build(project, caseEnv)
		})

		return image, err
	}
}

type ImageDesc struct {
	Dockerfile string
	ContextDir string
	Labels     StringsMap
	BuildArgs  StringsMap
}

func (i ImageDesc) build(project *ProjectEnv, testCase *TestCaseEnv) (string, error) {
	labels, err := i.Labels.resolve(project, testCase)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve labels")
	}

	buildArgs, err := i.Labels.resolve(project, testCase)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve build args")
	}

	imageName, err := project.client.BuildImage(docker.BuildImageParams{
		Dockerfile: i.Dockerfile,
		ContextDir: i.ContextDir,
		Labels:     labels,
		BuildArgs:  buildArgs,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return imageName, nil
}
