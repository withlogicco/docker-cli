package swarm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/stack/options"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// RunRemove is the swarm implementation of docker stack remove
func RunRemove(dockerCli command.Cli, opts options.Remove) error {
	client := dockerCli.Client()
	ctx := context.Background()

	var errs []string
	for _, namespace := range opts.Namespaces {
		services, err := getStackServices(ctx, client, namespace)
		if err != nil {
			return err
		}

		networks, err := getStackNetworks(ctx, client, namespace)
		if err != nil {
			return err
		}

		var secrets []swarm.Secret
		if versions.GreaterThanOrEqualTo(client.ClientVersion(), "1.25") {
			secrets, err = getStackSecrets(ctx, client, namespace)
			if err != nil {
				return err
			}
		}

		var configs []swarm.Config
		if versions.GreaterThanOrEqualTo(client.ClientVersion(), "1.30") {
			configs, err = getStackConfigs(ctx, client, namespace)
			if err != nil {
				return err
			}
		}

		if len(services)+len(networks)+len(secrets)+len(configs) == 0 {
			fmt.Fprintf(dockerCli.Err(), "Nothing found in stack: %s\n", namespace)
			continue
		}

		hasError := removeServices(ctx, dockerCli, services)
		hasError = removeSecrets(ctx, dockerCli, secrets) || hasError
		hasError = removeConfigs(ctx, dockerCli, configs) || hasError
		hasError = removeNetworks(ctx, dockerCli, networks) || hasError

		if hasError {
			errs = append(errs, fmt.Sprintf("Failed to remove some resources from stack: %s", namespace))
		}

		if !opts.Detach {
			err = waitOnTasks(ctx, client, namespace)
			if err != nil {
				fmt.Println(dockerCli.Err(), err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Errorf(strings.Join(errs, "\n"))
	}
	return nil
}

func sortServiceByName(services []swarm.Service) func(i, j int) bool {
	return func(i, j int) bool {
		return services[i].Spec.Name < services[j].Spec.Name
	}
}

func removeServices(
	ctx context.Context,
	dockerCli command.Cli,
	services []swarm.Service,
) bool {
	var hasError bool
	sort.Slice(services, sortServiceByName(services))
	for _, service := range services {
		fmt.Fprintf(dockerCli.Out(), "Removing service %s\n", service.Spec.Name)
		if err := dockerCli.Client().ServiceRemove(ctx, service.ID); err != nil {
			hasError = true
			fmt.Fprintf(dockerCli.Err(), "Failed to remove service %s: %s", service.ID, err)
		}
	}
	return hasError
}

func removeNetworks(
	ctx context.Context,
	dockerCli command.Cli,
	networks []types.NetworkResource,
) bool {
	var hasError bool
	for _, network := range networks {
		fmt.Fprintf(dockerCli.Out(), "Removing network %s\n", network.Name)
		if err := dockerCli.Client().NetworkRemove(ctx, network.ID); err != nil {
			hasError = true
			fmt.Fprintf(dockerCli.Err(), "Failed to remove network %s: %s", network.ID, err)
		}
	}
	return hasError
}

func removeSecrets(
	ctx context.Context,
	dockerCli command.Cli,
	secrets []swarm.Secret,
) bool {
	var hasError bool
	for _, secret := range secrets {
		fmt.Fprintf(dockerCli.Out(), "Removing secret %s\n", secret.Spec.Name)
		if err := dockerCli.Client().SecretRemove(ctx, secret.ID); err != nil {
			hasError = true
			fmt.Fprintf(dockerCli.Err(), "Failed to remove secret %s: %s", secret.ID, err)
		}
	}
	return hasError
}

func removeConfigs(
	ctx context.Context,
	dockerCli command.Cli,
	configs []swarm.Config,
) bool {
	var hasError bool
	for _, config := range configs {
		fmt.Fprintf(dockerCli.Out(), "Removing config %s\n", config.Spec.Name)
		if err := dockerCli.Client().ConfigRemove(ctx, config.ID); err != nil {
			hasError = true
			fmt.Fprintf(dockerCli.Err(), "Failed to remove config %s: %s", config.ID, err)
		}
	}
	return hasError
}

var numberedStates = map[swarm.TaskState]int64{
	swarm.TaskStateNew:       1,
	swarm.TaskStateAllocated: 2,
	swarm.TaskStatePending:   3,
	swarm.TaskStateAssigned:  4,
	swarm.TaskStateAccepted:  5,
	swarm.TaskStatePreparing: 6,
	swarm.TaskStateReady:     7,
	swarm.TaskStateStarting:  8,
	swarm.TaskStateRunning:   9,
	swarm.TaskStateComplete:  10,
	swarm.TaskStateShutdown:  11,
	swarm.TaskStateFailed:    12,
	swarm.TaskStateRejected:  13,
}

func terminalState(state swarm.TaskState) bool {
	return numberedStates[state] > numberedStates[swarm.TaskStateRunning]
}

func waitOnTasks(ctx context.Context, apiclient client.APIClient, namespace string) error {
	terminalStatesReached := 0
	for {
		tasks, err := getStackTasks(ctx, apiclient, namespace)
		if err != nil {
			return fmt.Errorf("Failed to get tasks for stack: %s: %w", namespace, err)
		}

		for _, task := range tasks {
			if terminalState(task.Status.State) {
				terminalStatesReached++
				break
			}
		}

		if terminalStatesReached == len(tasks) {
			break
		}
	}
	return nil
}
