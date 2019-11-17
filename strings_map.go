package testenv

import (
	"os"

	"github.com/pkg/errors"
)

type StringValueResolver func(project *ProjectEnv, caseEnv *TestCaseEnv) (string, error)

func ProjectVariableValue(key string) StringValueResolver {
	return func(project *ProjectEnv, caseEnv *TestCaseEnv) (s string, e error) {
		value := project.Get(key)
		if value == nil {
			return "", errors.Errorf("no variable %s in project", key)
		}
		result, ok := value.(string)
		if !ok {
			return "", errors.Errorf("variable %s in project is not a string", key)
		}

		return result, nil
	}
}
func StringValue(value string) StringValueResolver {
	return func(project *ProjectEnv, caseEnv *TestCaseEnv) (s string, e error) {
		return value, nil
	}
}

func EnvStringValue(variable string) StringValueResolver {
	return func(project *ProjectEnv, caseEnv *TestCaseEnv) (s string, e error) {
		result, ok := os.LookupEnv(variable)
		if !ok {
			return "", errors.New("no env variable " + variable)
		}
		return result, nil
	}
}

type StringsMap map[string]StringValueResolver

func (l StringsMap) resolve(project *ProjectEnv, testCase *TestCaseEnv) (map[string]string, error) {
	labels := make(map[string]string)
	for key, valueResolver := range l {
		value, err := valueResolver(project, testCase)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve %s value", key)
		}
		labels[key] = value
	}

	return labels, nil
}
