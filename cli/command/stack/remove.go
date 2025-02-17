package stack

import (
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/stack/options"
	"github.com/docker/cli/cli/command/stack/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newRemoveCommand(dockerCli command.Cli) *cobra.Command {
	var opts options.Remove

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] STACK [STACK...]",
		Aliases: []string{"remove", "down"},
		Short:   "Remove one or more stacks",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Namespaces = args
			if err := validateStackNames(opts.Namespaces); err != nil {
				return err
			}
			return swarm.RunRemove(dockerCli, opts)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeNames(dockerCli)(cmd, args, toComplete)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Detach, "detach", "d", true, "Exit immediately instead of waiting for the stack tasks to converge")
	return cmd
}

// RunRemove performs a stack remove against the specified swarm cluster.
//
// Deprecated: use [swarm.RunRemove] instead.
func RunRemove(dockerCli command.Cli, _ *pflag.FlagSet, opts options.Remove) error {
	return swarm.RunRemove(dockerCli, opts)
}
