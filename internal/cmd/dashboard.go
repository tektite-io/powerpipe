package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/pipe-fittings/cloud"
	"github.com/turbot/pipe-fittings/cmdconfig"
	"github.com/turbot/pipe-fittings/constants"
	"github.com/turbot/pipe-fittings/contexthelpers"
	"github.com/turbot/pipe-fittings/error_helpers"
	"github.com/turbot/pipe-fittings/export"
	"github.com/turbot/pipe-fittings/modconfig"
	"github.com/turbot/pipe-fittings/schema"
	"github.com/turbot/pipe-fittings/statushooks"
	"github.com/turbot/pipe-fittings/steampipeconfig"
	"github.com/turbot/pipe-fittings/workspace"
	localcmdconfig "github.com/turbot/powerpipe/internal/cmdconfig"
	localconstants "github.com/turbot/powerpipe/internal/constants"
	"github.com/turbot/powerpipe/internal/controlstatus"
	"github.com/turbot/powerpipe/internal/dashboardexecute"
	"github.com/turbot/powerpipe/internal/initialisation"
	"github.com/turbot/steampipe-plugin-sdk/v5/logging"
)

func dashboardRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:              "run [flags] [dashboard]",
		TraverseChildren: true,
		Args:             cobra.ExactArgs(1),
		Run:              dashboardRun,
		Short:            "Run a named dashboard",
		Long: `Runs the named dashboard.

The current mod is the working directory, or the directory specified by the --mod-location flag.`,
	}

	cmdconfig.OnCmd(cmd).
		AddCloudFlags().
		AddWorkspaceDatabaseFlag().
		AddModLocationFlag().
		AddBoolFlag(constants.ArgHelp, false, "Help for dashboard", cmdconfig.FlagOptions.WithShortHand("h")).
		AddBoolFlag(constants.ArgModInstall, true, "Specify whether to install mod dependencies before running the dashboard").
		AddStringSliceFlag(constants.ArgSearchPath, nil, "Set a custom search_path for the steampipe user for a dashboard session (comma-separated)").
		AddStringSliceFlag(constants.ArgSearchPathPrefix, nil, "Set a prefix to the current search path for a dashboard session (comma-separated)").
		AddIntFlag(constants.ArgMaxParallel, constants.DefaultMaxConnections, "The maximum number of concurrent database connections to open").
		AddStringSliceFlag(constants.ArgVarFile, nil, "Specify an .spvar file containing variable values").
		AddBoolFlag(constants.ArgProgress, true, "Display dashboard execution progress respected when a dashboard name argument is passed").
		// NOTE: use StringArrayFlag for ArgVariable, not StringSliceFlag
		// Cobra will interpret values passed to a StringSliceFlag as CSV, where args passed to StringArrayFlag are not parsed and used raw
		AddStringArrayFlag(constants.ArgVariable, nil, "Specify the value of a variable").
		AddBoolFlag(constants.ArgInput, true, "Enable interactive prompts").
		// DO use enum
		AddStringFlag(constants.ArgOutput, constants.OutputFormatSnapshot, "Select a console output format: none, snapshot"). // TODO KAI available options in help
		AddBoolFlag(constants.ArgSnapshot, false, "Create snapshot in Turbot Pipes with the default (workspace) visibility").
		AddBoolFlag(constants.ArgShare, false, "Create snapshot in Turbot Pipes with 'anyone_with_link' visibility").
		AddStringFlag(constants.ArgSnapshotLocation, "", "The location to write snapshots - either a local file path or a Turbot Pipes workspace").
		AddStringFlag(constants.ArgSnapshotTitle, "", "The title to give a snapshot").
		// NOTE: use StringArrayFlag for ArgDashboardInput, not StringSliceFlag
		// Cobra will interpret values passed to a StringSliceFlag as CSV, where args passed to StringArrayFlag are not parsed and used raw
		AddStringArrayFlag(constants.ArgDashboardInput, nil, "Specify the value of a dashboard input").
		AddStringArrayFlag(constants.ArgSnapshotTag, nil, "Specify tags to set on the snapshot").
		AddStringSliceFlag(constants.ArgExport, nil, "Export output to file, supported format: sps (snapshot)").
		// hidden flags that are used internally
		AddBoolFlag(constants.ArgServiceMode, false, "Hidden flag to specify whether this is starting as a service", cmdconfig.FlagOptions.Hidden())

	return cmd
}

func dashboardRun(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	// there can only be a single arg - cobra will validate
	dashboardName := args[0]

	var err error
	logging.LogTime("dashboardRun start")
	defer func() {
		logging.LogTime("dashboardRun end")
		if r := recover(); r != nil {
			err = helpers.ToError(r)
			error_helpers.ShowError(ctx, err)

		}
		setExitCodeForDashboardError(err)
	}()

	// first check whether a single dashboard name has been passed as an arg
	error_helpers.FailOnError(validateDashboardArgs(ctx))

	// if diagnostic mode is set, print out config and return
	if _, ok := os.LookupEnv(localconstants.EnvConfigDump); ok {
		localcmdconfig.DisplayConfig()
		return
	}

	inputs, err := collectInputs()
	error_helpers.FailOnError(err)

	// create context for the dashboard execution
	ctx = createSnapshotContext(ctx, dashboardName)

	statushooks.SetStatus(ctx, "Initializing…")
	initData := getInitData(ctx, dashboardName)

	statushooks.Done(ctx)

	// shutdown the service on exit
	defer initData.Cleanup(ctx)
	error_helpers.FailOnError(initData.Result.Error)

	// if there is a usage warning we display it
	initData.Result.DisplayMessages()

	// so a dashboard name was specified - just call GenerateSnapshot
	snap, err := dashboardexecute.GenerateSnapshot(ctx, initData, inputs)
	error_helpers.FailOnError(err)
	// display the snapshot result (if needed)
	displaySnapshot(snap)

	// upload the snapshot (if needed)
	err = publishSnapshotIfNeeded(ctx, snap)
	if err != nil {
		exitCode = constants.ExitCodeSnapshotUploadFailed
		error_helpers.FailOnErrorWithMessage(err, fmt.Sprintf("failed to publish snapshot to %s", viper.GetString(constants.ArgSnapshotLocation)))
	}

	// export the result (if needed)
	exportArgs := viper.GetStringSlice(constants.ArgExport)
	exportMsg, err := initData.ExportManager.DoExport(ctx, snap.FileNameRoot, snap, exportArgs)
	error_helpers.FailOnErrorWithMessage(err, "failed to export snapshot")

	// print the location where the file is exported
	if len(exportMsg) > 0 && viper.GetBool(constants.ArgProgress) {
		//nolint:forbidigo // Intentional UI output
		fmt.Printf("\n%s\n", strings.Join(exportMsg, "\n"))
	}

}

// validate the args and extract a dashboard name, if provided
func validateDashboardArgs(ctx context.Context) error {
	err := localcmdconfig.ValidateSnapshotArgs(ctx)
	if err != nil {
		return err
	}

	// only 1 of 'share' and 'snapshot' may be set
	share := viper.GetBool(constants.ArgShare)
	snapshot := viper.GetBool(constants.ArgSnapshot)
	if share && snapshot {
		return fmt.Errorf("only one of --share or --snapshot may be set")
	}

	validOutputFormats := []string{constants.OutputFormatSnapshot, constants.OutputFormatSnapshotShort, constants.OutputFormatNone}
	output := viper.GetString(constants.ArgOutput)
	if !helpers.StringSliceContains(validOutputFormats, output) {
		return fmt.Errorf("invalid output format: '%s', must be one of [%s]", output, strings.Join(validOutputFormats, ", "))
	}

	return nil
}

func displaySnapshot(snapshot *steampipeconfig.SteampipeSnapshot) {
	switch viper.GetString(constants.ArgOutput) {
	case constants.OutputFormatSnapshot, constants.OutputFormatSnapshotShort:
		// just display result
		snapshotText, err := json.MarshalIndent(snapshot, "", "  ")
		error_helpers.FailOnError(err)
		//nolint:forbidigo // Intentional UI output
		fmt.Println(string(snapshotText))
	}
}

func getInitData(ctx context.Context, dashboardName string) *initialisation.InitData {
	modLocation := viper.GetString(constants.ArgModLocation)

	w, errAndWarnings := workspace.LoadWorkspacePromptingForVariables(ctx, modLocation)
	if errAndWarnings.GetError() != nil {
		return initialisation.NewErrorInitData(fmt.Errorf("failed to load workspace: %s", error_helpers.HandleCancelError(errAndWarnings.GetError()).Error()))
	}

	i := initialisation.NewInitData()
	i.Workspace = w
	i.Result.Warnings = errAndWarnings.Warnings
	i.Init(ctx, "dashboard", dashboardName)

	if len(viper.GetStringSlice(constants.ArgExport)) > 0 {
		if err := i.RegisterExporters(dashboardExporters()...); err != nil {
			i.Result.Error = err
			return i
		}
		// validate required export formats
		if err := i.ExportManager.ValidateExportFormat(viper.GetStringSlice(constants.ArgExport)); err != nil {
			i.Result.Error = err
			return i
		}
	}

	return i
}

func dashboardExporters() []export.Exporter {
	return []export.Exporter{&export.SnapshotExporter{}}
}

func publishSnapshotIfNeeded(ctx context.Context, snapshot *steampipeconfig.SteampipeSnapshot) error {
	shouldShare := viper.GetBool(constants.ArgShare)
	shouldUpload := viper.GetBool(constants.ArgSnapshot)

	if !(shouldShare || shouldUpload) {
		return nil
	}

	message, err := cloud.PublishSnapshot(ctx, snapshot, shouldShare)
	if err != nil {
		// reword "402 Payment Required" error
		return handlePublishSnapshotError(err)
	}
	if viper.GetBool(constants.ArgProgress) {
		//nolint:forbidigo // Intentional UI output
		fmt.Println(message)
	}
	return nil
}

func handlePublishSnapshotError(err error) error {
	if err.Error() == "402 Payment Required" {
		return fmt.Errorf("maximum number of snapshots reached")
	}
	return err
}

func setExitCodeForDashboardError(err error) {
	// if exit code already set, leave as is
	if exitCode != 0 || err == nil {
		return
	}

	if errors.Is(err, workspace.ErrorNoModDefinition) {
		exitCode = constants.ExitCodeNoModFile
	} else {
		exitCode = constants.ExitCodeUnknownErrorPanic
	}
}

func collectInputs() (map[string]interface{}, error) {
	res := make(map[string]interface{})
	inputArgs := viper.GetStringSlice(constants.ArgDashboardInput)
	for _, variableArg := range inputArgs {
		// Value should be in the form "name=value", where value is a string
		raw := variableArg
		eq := strings.Index(raw, "=")
		if eq == -1 {
			return nil, fmt.Errorf("the --dashboard-input argument '%s' is not correctly specified. It must be an input name and value separated an equals sign: --dashboard-input key=value", raw)
		}
		name := raw[:eq]
		rawVal := raw[eq+1:]
		if _, ok := res[name]; ok {
			return nil, fmt.Errorf("the dashboard-input option '%s' is provided more than once", name)
		}
		// add `input. to start of name
		key := modconfig.BuildModResourceName(schema.BlockTypeInput, name)
		res[key] = rawVal
	}

	return res, nil

}

// create the context for the dashboard run - add a control status renderer
func createSnapshotContext(ctx context.Context, target string) context.Context {
	// create context for the dashboard execution
	snapshotCtx, cancel := context.WithCancel(ctx)
	contexthelpers.StartCancelHandler(cancel)

	// if progress is disabled, OR output is none, do not show status hooks
	if !viper.GetBool(constants.ArgProgress) {
		snapshotCtx = statushooks.DisableStatusHooks(snapshotCtx)
	}

	snapshotProgressReporter := statushooks.NewSnapshotProgressReporter(target)
	snapshotCtx = statushooks.AddSnapshotProgressToContext(snapshotCtx, snapshotProgressReporter)

	// create a context with a SnapshotControlHooks to report execution progress of any controls in this snapshot
	snapshotCtx = controlstatus.AddControlHooksToContext(snapshotCtx, controlstatus.NewSnapshotControlHooks())
	return snapshotCtx
}
