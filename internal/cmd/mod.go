package cmd

import (
	"github.com/spf13/cobra"
	"github.com/turbot/steampipe/pkg/cmdconfig"
	"github.com/turbot/steampipe/pkg/constants"
)

func modCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "mod [command]",
		Args:  cobra.NoArgs,
		Short: "Powerpipe mod management",
		Long: `Powerpipe mod management.

Mods enable you to run, build, and share dashboards, benchmarks and other resources.

Find pre-built mods in the public registry at https://hub.steampipe.io.

Examples:

    # Create a new mod in the current directory
    powerpipe mod init

    # Install a mod
    powerpipe mod install github.com/turbot/steampipe-mod-aws-compliance
    
    # Update a mod
    powerpipe mod update github.com/turbot/steampipe-mod-aws-compliance
    
    # List installed mods
    powerpipe mod list
    
    # Uninstall a mod
    powerpipe mod uninstall github.com/turbot/steampipe-mod-aws-compliance
	`,
	}
	cmd.AddCommand(modInstallCmd())
	cmd.AddCommand(modUninstallCmd())
	cmd.AddCommand(modUpdateCmd())
	cmd.AddCommand(modListCmd())
	cmd.AddCommand(modInitCmd())
	cmd.Flags().BoolP("help", "h", false, "Help for mod")

	return cmd
}

func modInstallCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "install",
		Run:   runModInstallCmd,
		Short: "Install one or more mods and their dependencies",
		Long: `Install one or more mods and their dependencies.

Mods provide an easy way to share Steampipe queries, controls, and benchmarks.
Find mods using the public registry at hub.steampipe.io.

Examples:

  # Install a mod(steampipe-mod-aws-compliance)
  powerpipe mod install github.com/turbot/steampipe-mod-aws-compliance

  # Install a specific version of a mod
  powerpipe mod install github.com/turbot/steampipe-mod-aws-compliance@0.1

  # Install a version of a mod using a semver constraint
  powerpipe mod install github.com/turbot/steampipe-mod-aws-compliance@'^1'

  # Install all mods specified in the mod.sp and their dependencies
  powerpipe mod install

  # Preview what powerpipw mod install will do, without actually installing anything
  powerpipe mod install --dry-run`,
	}

	cmdconfig.OnCmd(cmd).
		AddBoolFlag(constants.ArgPrune, true, "Remove unused dependencies after installation is complete").
		AddBoolFlag(constants.ArgDryRun, false, "Show which mods would be installed/updated/uninstalled without modifying them").
		AddBoolFlag(constants.ArgForce, false, "Install mods even if plugin/cli version requirements are not met (cannot be used with --dry-run)").
		AddBoolFlag(constants.ArgHelp, false, "Help for install", cmdconfig.FlagOptions.WithShortHand("h")).
		AddModLocationFlag()

	return cmd
}

func runModInstallCmd(cmd *cobra.Command, args []string) {}

func modUninstallCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "uninstall",
		Run:   runModUninstallCmd,
		Short: "Uninstall a mod and its dependencies",
		Long: `Uninstall a mod and its dependencies.

Example:
  
  # Uninstall a mod
  powerpipe mod uninstall github.com/turbot/steampipe-mod-azure-compliance`,
	}

	cmdconfig.OnCmd(cmd).
		AddBoolFlag(constants.ArgPrune, true, "Remove unused dependencies after uninstallation is complete").
		AddBoolFlag(constants.ArgDryRun, false, "Show which mods would be uninstalled without modifying them").
		AddBoolFlag(constants.ArgHelp, false, "Help for uninstall", cmdconfig.FlagOptions.WithShortHand("h")).
		AddModLocationFlag()

	return cmd
}

func runModUninstallCmd(cmd *cobra.Command, args []string) {}

func modUpdateCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "update",
		Run:   runModUpdateCmd,
		Short: "Update one or more mods and their dependencies",
		Long: `Update one or more mods and their dependencies.

Example:

  # Update a mod to the latest version allowed by its current constraint
  powerpipe mod update github.com/turbot/steampipe-mod-aws-compliance

  # Update all mods specified in the mod.sp and their dependencies to the latest versions that meet their constraints, and install any that are missing
  powerpipe mod update`,
	}

	cmdconfig.OnCmd(cmd).
		AddBoolFlag(constants.ArgPrune, true, "Remove unused dependencies after update is complete").
		AddBoolFlag(constants.ArgForce, false, "Update mods even if plugin/cli version requirements are not met (cannot be used with --dry-run)").
		AddBoolFlag(constants.ArgDryRun, false, "Show which mods would be updated without modifying them").
		AddBoolFlag(constants.ArgHelp, false, "Help for update", cmdconfig.FlagOptions.WithShortHand("h")).
		AddModLocationFlag()

	return cmd
}

func runModUpdateCmd(cmd *cobra.Command, args []string) {}

func modListCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "list",
		Run:   runModListCmd,
		Short: "List currently installed mods",
		Long: `List currently installed mods.
		
Example:

  # List installed mods
  powerpipe mod list`,
	}

	cmdconfig.OnCmd(cmd).
		AddBoolFlag(constants.ArgHelp, false, "Help for list", cmdconfig.FlagOptions.WithShortHand("h")).
		AddModLocationFlag()
	return cmd
}

func runModListCmd(cmd *cobra.Command, _ []string) {}

func modInitCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "init",
		Run:   runModInitCmd,
		Short: "Initialize the current directory with a mod.sp file",
		Long: `Initialize the current directory with a mod.sp file.
		
Example:

  # Initialize the current directory with a mod.sp file
  powerpipe mod init`,
	}

	cmdconfig.OnCmd(cmd).
		AddBoolFlag(constants.ArgHelp, false, "Help for init", cmdconfig.FlagOptions.WithShortHand("h")).
		AddModLocationFlag()
	return cmd
}

func runModInitCmd(cmd *cobra.Command, args []string) {}
