package example

import (
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/saturn4er/go-testenv"
	"github.com/stretchr/testify/require"
)

const (
	publicNetworkName   = "public_network"
	testCaseNetworkName = "test_case"

	kafkaImage     = "wurstmeister/kafka:2.11-1.1.1"
	zookeeperImage = "zookeeper:3.4.13"

	hostIP       = "192.168.0.153"
	testEnvLabel = "test_env"
)

var testEnvID = uuid.NewV4().String()

var testEnv = testenv.ProjectEnvDesc{
	Networks: map[string]testenv.NetworkDesc{
		publicNetworkName: {
			Labels: map[string]testenv.StringValueResolver{
				testEnvLabel: testenv.StringValue(testEnvID),
			},
		},
	},
	Containers: map[string]testenv.ContainerDesc{
		"kafka": {
			Hooks: testenv.ContainerHooks{
				BeforeRun: func(project *testenv.ProjectEnv, testCase *testenv.TestCaseEnv) error {
					kafkaHostPort, err := project.FindFreeHostPort()
					if err != nil {
						return errors.WithStack(err)
					}
					project.Set("kafka_host_port", kafkaHostPort)

					return nil
				},
			},
			Image:        testenv.ExternalImage(kafkaImage),
			ExposedPorts: []testenv.StringValueResolver{testenv.ProjectVariableValue("kafka_host_port")},
			Labels: map[string]testenv.StringValueResolver{
				testEnvLabel: testenv.StringValue(testEnvID),
			},
			PortBindings: []testenv.PortBinding{
				{
					ContainerPort: testenv.ProjectVariableValue("kafka_host_port"),
					Host:          testenv.StringValue(hostIP),
					Port:          testenv.ProjectVariableValue("kafka_host_port"),
				},
			},
			Envs: map[string]testenv.StringValueResolver{
				"KAFKA_BROKER_ID":                      testenv.StringValue("1"),
				"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": testenv.StringValue("INSIDE:PLAINTEXT,OUTSIDE:PLAINTEXT"),
				"KAFKA_LISTENERS": func(project *testenv.ProjectEnv, caseEnv *testenv.TestCaseEnv) (s string, e error) {
					kafkaHostPort := project.Get("kafka_host_port").(string)
					return "INSIDE://:9092,OUTSIDE://:" + kafkaHostPort, nil
				},
				"KAFKA_ADVERTISED_LISTENERS": func(project *testenv.ProjectEnv, caseEnv *testenv.TestCaseEnv) (s string, e error) {
					kafkaHostPort := project.Get("kafka_host_port").(string)
					return fmt.Sprintf("INSIDE://:9092,OUTSIDE://%s:%s", hostIP, kafkaHostPort), nil
				},
				"KAFKA_INTER_BROKER_LISTENER_NAME": testenv.StringValue("INSIDE"),
				"KAFKA_ZOOKEEPER_CONNECT":          testenv.StringValue("zookeeper:2181"),
				"KAFKA_AUTO_CREATE_TOPICS_ENABLE":  testenv.StringValue("true"),
			},
			Networks: []testenv.ContainerNetwork{
				{
					Network: testenv.ProjectNetwork(publicNetworkName),
					Alias:   "kafka",
				},
			},
		},
		"zookeeper": {
			Image: testenv.ExternalImage(zookeeperImage),
			Labels: map[string]testenv.StringValueResolver{
				testEnvLabel: testenv.StringValue(testEnvID),
			},
			Networks: []testenv.ContainerNetwork{
				{
					Network: testenv.ProjectNetwork(publicNetworkName),
					Alias:   "zookeeper",
				},
			},
		},
	},
	TestCaseEnv: testenv.TestCaseEnvDesc{
		Networks: map[string]testenv.NetworkDesc{
			testCaseNetworkName: {
				Labels: map[string]testenv.StringValueResolver{
					testEnvLabel: testenv.StringValue(testEnvID),
				},
			},
		},
		Containers: map[string]*testenv.ContainerDesc{
			"postgres": {
				Image:        testenv.ExternalImage("postgres:latest"),
				ExposedPorts: []testenv.StringValueResolver{testenv.StringValue("5432")},
				Networks: []testenv.ContainerNetwork{
					{
						Network: testenv.TestCaseNetwork(testCaseNetworkName),
						Alias:   "postgres",
					},
				},
			},
			"server": {
				Image: testenv.BuildImage(testenv.ImageDesc{
					Dockerfile: "./Dockerfile",
					ContextDir: "./docker",
					Labels: map[string]testenv.StringValueResolver{
						testEnvLabel: testenv.StringValue(testEnvID),
					},
				}),
				Networks: []testenv.ContainerNetwork{
					{
						Network: testenv.ProjectNetwork(publicNetworkName),
					},
					{
						Network: testenv.TestCaseNetwork(testCaseNetworkName),
						Alias:   "server",
					},
				},
			},
		},
	},
}

func TestEnv(t *testing.T) {
	projectEnv, err := testenv.NewProjectEnv(testEnv)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, projectEnv.Close())
	}()

	err = projectEnv.Run()
	require.NoError(t, err)

	tcEnv := projectEnv.NewTestCaseEnv()
	defer func() {
		require.NoError(t, tcEnv.Close())
	}()

	err = tcEnv.Run()
	require.NoError(t, err)

	time.Sleep(time.Second * 20)
}
