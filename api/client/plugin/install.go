// +build experimental

package plugin

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type pluginOptions struct {
	name       string
	grantPerms bool
}

func newInstallCommand(dockerCli *client.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install a plugin",
		Args:  cli.RequiresMinArgs(1), // TODO: allow for set args
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return runInstall(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&options.grantPerms, "grant-all-permissions", true, "grant all permissions necessary to run the plugin")

	return cmd
}

func runInstall(dockerCli *client.DockerCli, opts pluginOptions) error {
	named, err := reference.ParseNamed(opts.name) // FIXME: validate
	if err != nil {
		return err
	}
	named = reference.WithDefaultTag(named)
	ref, ok := named.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("invalid name: %s", named.String())
	}

	ctx := context.Background()

	repoInfo, err := registry.ParseRepositoryInfo(named)
	authConfig := dockerCli.ResolveAuthConfig(ctx, repoInfo.Index)

	encodedAuth, err := client.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	requestPrivilege := dockerCli.RegistryAuthenticationPrivilegedFunc(repoInfo.Index, "plugin install")

	// TODO: pass acceptAllPermissions and noEnable flag
	options := types.PluginInstallOptions{
		RegistryAuth:          encodedAuth,
		Disabled:              false,
		AcceptAllPermissions:  opts.grantPerms,
		AcceptPermissionsFunc: acceptPrivileges(dockerCli, opts.name),
		PrivilegeFunc:         requestPrivilege,
	}

	return dockerCli.Client().PluginInstall(ctx, ref.String(), options)
}

func acceptPrivileges(dockerCli *client.DockerCli, name string) func(privileges types.PluginPrivileges) (bool, error) {
	return func(privileges types.PluginPrivileges) (bool, error) {
		fmt.Fprintf(dockerCli.Out(), "Plugin %q requested the following privileges:\n", name)
		for _, privilege := range privileges {
			fmt.Fprintf(dockerCli.Out(), " - %s: %v\n", privilege.Name, privilege.Value)
		}

		fmt.Fprint(dockerCli.Out(), "Do you grant the above permissions? [y/N] ")
		reader := bufio.NewReader(dockerCli.In())
		line, _, err := reader.ReadLine()
		if err != nil {
			return false, err
		}
		return strings.ToLower(string(line)) == "y", nil
	}
}
