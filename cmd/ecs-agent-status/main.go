package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/natemarks/ecs-agent-status/version"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/rs/zerolog"
)

// Agent is a struct that contains information about an ECS agent
type Agent struct {
	Cluster              string `json:"cluster"`
	ContainerInstanceARN string `json:"containerInstanceArn"`
	EC2InstanceID        string `json:"ec2InstanceId"`
	AgentStatus          string `json:"agentStatus"`
}

func (a Agent) String() string {
	return fmt.Sprintf("Cluster: %v, ContainerInstanceARN: %v, EC2InstanceID: %v, AgentStatus: %v", a.Cluster, a.ContainerInstanceARN, a.EC2InstanceID, a.AgentStatus)
}

// GetInput returns the value of the first positional argument to be used as the substring
// to match cluster names
func GetInput() string {
	args := os.Args[1:] // Retrieve all command-line arguments except the program name (index 0)

	// Check if at least one argument is provided
	if len(args) < 1 {
		fmt.Println("Usage: ecs-agent-status <cluster name substring>")
		os.Exit(1)
	}

	// Return the value of the first positional argument
	return args[0]
}

// GetECSClustersWithSubstring returns a list of ECS cluster names that contain the specified substring
func GetECSClustersWithSubstring(substring string) ([]string, error) {
	var clusters []string

	// Load AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	// Create an ECS client
	client := ecs.NewFromConfig(cfg)

	// Initialize paginator for ListClusters API
	paginator := ecs.NewListClustersPaginator(client, &ecs.ListClustersInput{})

	// Iterate through pages of clusters
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		// Check if cluster names contain the specified substring
		for _, clusterArn := range output.ClusterArns {
			clusterName := strings.Split(clusterArn, "/")[1] // Extract cluster name from ARN
			if strings.Contains(clusterName, substring) {
				clusters = append(clusters, clusterName)
			}
		}
	}
	if len(clusters) == 0 {
		return nil, errors.New("no clusters found")
	}
	return clusters, nil
}

// GetContainerInstancesForCluster returns a list of container instance ARNs for the specified ECS cluster
func GetContainerInstancesForCluster(clusterName string) ([]string, error) {
	var containerInstances []string

	// Load AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	// Create an ECS client
	client := ecs.NewFromConfig(cfg)

	// Initialize the input parameters for ListContainerInstances API
	input := &ecs.ListContainerInstancesInput{
		Cluster: &clusterName,
	}

	// Retrieve the list of container instances for the specified ECS cluster
	output, err := client.ListContainerInstances(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(output.ContainerInstanceArns) == 0 {
		return nil, errors.New("no container instances found")
	}
	// Describe container instances to get their ARNs
	describeInput := &ecs.DescribeContainerInstancesInput{
		Cluster:            &clusterName,
		ContainerInstances: output.ContainerInstanceArns,
	}

	describeOutput, err := client.DescribeContainerInstances(context.Background(), describeInput)
	if err != nil {
		return nil, err
	}

	// Extract the ARNs of container instances
	for _, instance := range describeOutput.ContainerInstances {
		containerInstances = append(containerInstances, *instance.ContainerInstanceArn)
	}

	return containerInstances, nil
}

// GetEC2InstanceIDAndECSAgentStatus returns the EC2 instance ID and ECS agent status for the specified
// container instance
func GetEC2InstanceIDAndECSAgentStatus(clusterName, containerInstanceArn string) (string, string, error) {
	var ec2InstanceID, ecsAgentStatus string

	// Load AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", "", err
	}

	// Create an ECS client
	client := ecs.NewFromConfig(cfg)

	// Describe the container instance to retrieve ECS agent status
	describeInput := &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(clusterName),
		ContainerInstances: []string{containerInstanceArn},
	}

	describeOutput, err := client.DescribeContainerInstances(context.Background(), describeInput)
	if err != nil {
		return "", "", err
	}

	// Check if the container instance information exists
	if len(describeOutput.ContainerInstances) == 0 {
		return "", "", fmt.Errorf("container instance not found")
	}

	// Extract EC2 instance ID and ECS agent status
	ec2InstanceID = *describeOutput.ContainerInstances[0].Ec2InstanceId
	ecsAgentStatus = *describeOutput.ContainerInstances[0].Status

	return ec2InstanceID, ecsAgentStatus, nil
}

// GetAgentStatusForCluster returns a list of Agent structs for the specified ECS cluster
func GetAgentStatusForCluster(clusterName string) ([]Agent, error) {
	var agents []Agent

	// Get the list of container instances for the specified ECS cluster
	containerInstances, err := GetContainerInstancesForCluster(clusterName)
	if err != nil {
		return nil, err
	}

	// Get the EC2 instance ID and ECS agent status for each container instance
	for _, containerInstance := range containerInstances {
		ec2InstanceID, ecsAgentStatus, err := GetEC2InstanceIDAndECSAgentStatus(clusterName, containerInstance)
		if err != nil {
			return nil, err
		}

		// Create an Agent struct for the container instance
		agent := Agent{
			Cluster:              clusterName,
			ContainerInstanceARN: containerInstance,
			EC2InstanceID:        ec2InstanceID,
			AgentStatus:          ecsAgentStatus,
		}

		// Append the Agent struct to the list of agents
		agents = append(agents, agent)
	}

	return agents, nil
}
func main() {
	failed := false
	var agents []Agent
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	logger := zerolog.New(os.Stderr).With().Str("version", version.Version).Timestamp().Logger()
	clusterNameSubstring := GetInput()
	clusters, err := GetECSClustersWithSubstring(clusterNameSubstring)
	if err != nil {
		logger.Fatal().Err(err).Msgf("error getting clusters: %v", err)
	}
	logger.Info().Msgf("found %v matching clusters", len(clusters))
	for _, cluster := range clusters {
		result, err := GetAgentStatusForCluster(cluster)
		if err != nil {
			logger.Error().Err(err).Msgf("error getting agents for cluster %v: %v", cluster, err)
			continue
		}
		for _, agent := range result {
			agents = append(agents, agent)
		}
	}
	for _, agent := range agents {
		if agent.AgentStatus != "ACTIVE" {
			failed = true
		}
		fmt.Println(agent)
	}
	if failed {
		os.Exit(1)
	}
}
