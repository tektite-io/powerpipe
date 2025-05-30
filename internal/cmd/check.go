package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thediveo/enumflag/v2"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/pipe-fittings/v2/cmdconfig"
	"github.com/turbot/pipe-fittings/v2/constants"
	"github.com/turbot/pipe-fittings/v2/contexthelpers"
	"github.com/turbot/pipe-fittings/v2/error_helpers"
	"github.com/turbot/pipe-fittings/v2/statushooks"
	"github.com/turbot/pipe-fittings/v2/utils"
	localcmdconfig "github.com/turbot/powerpipe/internal/cmdconfig"
	localconstants "github.com/turbot/powerpipe/internal/constants"
	"github.com/turbot/powerpipe/internal/controldisplay"
	"github.com/turbot/powerpipe/internal/controlexecute"
	"github.com/turbot/powerpipe/internal/controlinit"
	"github.com/turbot/powerpipe/internal/controlstatus"
	"github.com/turbot/powerpipe/internal/dashboardexecute"
	"github.com/turbot/powerpipe/internal/db_client"
	"github.com/turbot/powerpipe/internal/display"
	localqueryresult "github.com/turbot/powerpipe/internal/queryresult"
	"github.com/turbot/powerpipe/internal/resources"
	"github.com/turbot/steampipe-plugin-sdk/v5/sperr"
)

// variable used to assign the output mode flag
var checkOutputMode = localconstants.CheckOutputModeText

// generic command to handle benchmark and control execution
func checkCmd[T controlinit.CheckTarget]() *cobra.Command {
	typeName := resources.GenericTypeToBlockType[T]()
	argsSupported := cobra.ExactArgs(1)
	if typeName == "benchmark" {
		argsSupported = cobra.MinimumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:              checkCmdUse(typeName),
		TraverseChildren: true,
		Args:             argsSupported,
		Run:              runCheckCmd[T],
		Short:            checkCmdShort(typeName),
		Long:             checkCmdLong(typeName),
	}

	// when running mod install before the benchmark execution, we use the minimal update strategy
	var updateStrategy = constants.ModUpdateIdMinimal

	builder := cmdconfig.OnCmd(cmd)
	builder.
		AddCloudFlags().
		AddModLocationFlag().
		AddStringFlag(constants.ArgDatabase, "", "Turbot Pipes workspace database", localcmdconfig.Deprecated("see https://powerpipe.io/docs/run#selecting-a-database for the new syntax")).
		AddBoolFlag(constants.ArgHeader, true, "Include column headers for csv and table output").
		AddBoolFlag(constants.ArgHelp, false, "Help for run command", cmdconfig.FlagOptions.WithShortHand("h")).
		AddBoolFlag(constants.ArgInput, true, "Enable interactive prompts").
		AddBoolFlag(constants.ArgModInstall, true, "Specify whether to install mod dependencies before running").
		AddVarFlag(enumflag.New(&updateStrategy, constants.ArgPull, constants.ModUpdateStrategyIds, enumflag.EnumCaseInsensitive),
			constants.ArgPull,
			fmt.Sprintf("Update strategy; one of: %s", strings.Join(constants.FlagValues(constants.ModUpdateStrategyIds), ", "))).
		AddBoolFlag(constants.ArgProgress, true, "Display control execution progress").
		AddBoolFlag(constants.ArgShare, false, "Create snapshot in Turbot Pipes with 'anyone_with_link' visibility").
		AddBoolFlag(constants.ArgSnapshot, false, "Create snapshot in Turbot Pipes with the default (workspace) visibility").
		AddBoolFlag(constants.ArgTiming, false, "Turn on the query timer").
		AddIntFlag(constants.ArgDatabaseQueryTimeout, localconstants.DatabaseDefaultQueryTimeout, "The query timeout").
		// NOTE: use StringArrayFlag for ArgVariable, not StringSliceFlag
		// Cobra will interpret values passed to a StringSliceFlag as CSV, where args passed to StringArrayFlag are not parsed and used raw
		AddStringArrayFlag(constants.ArgSnapshotTag, nil, "Specify tags to set on the snapshot").
		AddStringArrayFlag(constants.ArgVariable, nil, "Specify the value of a variable").
		AddStringArrayFlag(constants.ArgVarFile, nil, "Specify an .ppvar file containing variable values").
		// Define the CLI flag parameters for wrapped enum flag.
		AddVarFlag(enumflag.New(&checkOutputMode, constants.ArgOutput, localconstants.CheckOutputModeIds, enumflag.EnumCaseInsensitive),
			constants.ArgOutput,
			fmt.Sprintf("Output format; one of: %s", strings.Join(constants.FlagValues(localconstants.CheckOutputModeIds), ", "))).
		AddStringFlag(constants.ArgSeparator, ",", "Separator string for csv output").
		AddStringFlag(constants.ArgSnapshotLocation, "", "The location to write snapshots - either a local file path or a Turbot Pipes workspace").
		AddStringFlag(constants.ArgSnapshotTitle, "", "The title to give a snapshot").
		AddStringSliceFlag(constants.ArgExport, nil, "Export output to file, supported formats: csv, html, json, md, nunit3, pps (snapshot), asff").
		AddStringSliceFlag(constants.ArgSearchPath, nil, "Set a custom search_path (comma-separated)").
		AddStringSliceFlag(constants.ArgSearchPathPrefix, nil, "Set a prefix to the current search path (comma-separated)").
		AddIntFlag(constants.ArgBenchmarkTimeout, 0, "Set the benchmark execution timeout")

	// for control command, add --arg
	switch typeName {
	case "control":
		builder.AddStringArrayFlag(constants.ArgArg, nil, "Specify the value of a control argument")
	case "benchmark":
		builder.
			AddStringFlag(constants.ArgWhere, "", "SQL 'where' clause, or named query, used to filter controls (cannot be used with '--tag')").
			AddBoolFlag(constants.ArgDryRun, false, "Show which controls will be run without running them").
			AddStringSliceFlag(constants.ArgTag, nil, "Filter controls based on their tag values ('--tag key=value')").
			AddIntFlag(constants.ArgMaxParallel, constants.DefaultMaxConnections, "The maximum number of concurrent database connections to open")
	}

	return cmd
}

func checkCmdUse(typeName string) string {
	return fmt.Sprintf("run [flags] [%s]", typeName)
}
func checkCmdShort(typeName string) string {
	return fmt.Sprintf("Execute one or more %ss", typeName)
}
func checkCmdLong(typeName string) string {
	return fmt.Sprintf(`Execute one or more %ss.

You may specify one or more %ss to run, separated by a space.`, typeName, typeName)
}

// exitCode=0 no runtime errors, no control alarms or errors
// exitCode=1 no runtime errors, 1 or more control alarms, no control errors
// exitCode=2 no runtime errors, 1 or more control errors
// exitCode=3+ runtime errors

func runCheckCmd[T controlinit.CheckTarget](cmd *cobra.Command, args []string) {
	utils.LogTime("runCheckCmd start")

	startTime := time.Now()

	// setup a cancel context with timeout and start cancel handler
	var cancel context.CancelFunc
	var ctx context.Context
	ctx, cancel = context.WithCancel(cmd.Context())
	contexthelpers.StartCancelHandler(cancel)

	defer func() {
		utils.LogTime("runCheckCmd end")
		if r := recover(); r != nil {
			error_helpers.ShowError(ctx, helpers.ToError(r))
			exitCode = constants.ExitCodeUnknownErrorPanic
		}
	}()

	// validate the arguments
	err := validateCheckArgs(ctx)
	if err != nil {
		exitCode = constants.ExitCodeInsufficientOrWrongInputs
		error_helpers.ShowError(ctx, err)
		return
	}
	// if diagnostic mode is set, print out config and return
	if _, ok := os.LookupEnv(localconstants.EnvConfigDump); ok {
		localcmdconfig.DisplayConfig()
		return
	}

	// show the status spinner
	statushooks.Show(ctx)

	// disable status hooks in init - otherwise we will end up getting status updates all the way down from the service layer
	initCtx := statushooks.DisableStatusHooks(ctx)

	// initialise
	initData := controlinit.NewInitData[T](initCtx, cmd, args...)
	if initData.Result.Error != nil {
		exitCode = constants.ExitCodeInitializationFailed
		error_helpers.ShowError(ctx, initData.Result.Error)
		return
	}

	defer initData.Cleanup(ctx)

	// TODO TACTICAL
	// ifd the target is a detection benchmark, we need to run the detection benchmark using detectionRunWithInitData
	if _, ok := initData.Targets[0].(*resources.DetectionBenchmark); ok {
		if !viper.IsSet(constants.ArgOutput) {
			viper.Set(constants.ArgOutput, constants.OutputFormatSnapshot)
		}
		detectionRunWithInitData[*resources.DetectionBenchmark](cmd, initData, args)
		return
	}

	// hide the spinner so that warning messages can be shown
	statushooks.Done(ctx)

	// if there is a usage warning we display it
	initData.Result.DisplayMessages()

	// create a client to pass to the execution tree
	client, err := initData.GetDefaultClient(ctx)
	if err != nil {
		exitCode = constants.ExitCodeInitializationFailed
		error_helpers.ShowError(ctx, err)
		return

	}

	// get the execution trees
	trees, err := getExecutionTrees(ctx, initData, client)
	error_helpers.FailOnError(err)

	// pull out useful properties
	totalAlarms, totalErrors := 0, 0
	defer func() {
		// set the defined exit code after successful execution
		exitCode = getExitCode(totalAlarms, totalErrors)
		// close the database client
		if err := client.Close(ctx); err != nil {
			slog.Error("error closing database client", "error", err)
		}
	}()

	for _, namedTree := range trees {
		// execute controls synchronously (execute returns the number of alarms and errors)
		err = executeTree(ctx, namedTree.tree, initData)
		if err != nil {
			totalErrors++
			error_helpers.ShowError(ctx, err)
			return
		}

		// append the total number of alarms and errors for multiple runs
		totalAlarms = namedTree.tree.Root.Summary.Status.Alarm
		totalErrors = namedTree.tree.Root.Summary.Status.Error

		err = publishSnapshot(ctx, namedTree.tree, viper.GetBool(constants.ArgShare), viper.GetBool(constants.ArgSnapshot))
		if err != nil {
			error_helpers.ShowError(ctx, err)
			totalErrors++
			return
		}
		if shouldPrintCheckTiming() {
			display.PrintTiming(&localqueryresult.CheckTimingMetadata{
				Duration: time.Since(startTime),
			})
		}

		err = exportExecutionTree(ctx, namedTree, initData, viper.GetStringSlice(constants.ArgExport))
		if err != nil {
			error_helpers.ShowError(ctx, err)
			totalErrors++
		}
	}
}

// exportExecutionTree relies on the fact that the given tree is already executed
func exportExecutionTree(ctx context.Context, namedTree *namedExecutionTree, initData *controlinit.InitData, exportArgs []string) error {
	statushooks.Show(ctx)
	defer statushooks.Done(ctx)

	if error_helpers.IsContextCanceled(ctx) {
		return ctx.Err()
	}

	exportMsg, err := initData.ExportManager.DoExport(ctx, namedTree.name, namedTree.tree, exportArgs)
	if err != nil {
		return err
	}

	// print the location where the file is exported if progress=true
	if len(exportMsg) > 0 && viper.GetBool(constants.ArgProgress) {
		fmt.Printf("\n%s\n", strings.Join(exportMsg, "\n")) //nolint:forbidigo // we want to print
	}

	return nil
}

// executeTree executes and displays the (table) results of an execution
func executeTree(ctx context.Context, tree *controlexecute.ExecutionTree, initData *controlinit.InitData) error {
	// create a context with check status hooks
	checkCtx, cancel := createCheckContext(ctx)
	defer cancel()

	err := tree.Execute(checkCtx)
	if err != nil {
		return err
	}

	// populate the control run instances
	// if a control is included by multiple benchmarks, a single ControlRun is created, and executed only once,
	// and the Parents property contains a list of all ResultGroups (i.e. benchmarks) which include the control.
	// When rendering the CSV data, the template renders a set of results rows for every instance of the control,
	// i.e. for every parent.
	// So - build a list of ControlRunInstances, by expanding the list of control runs for each parent.
	// (A ControlRunInstance is the same as a ControlRun but has a single parent - thus if a ControlRun has 3 parents,
	// we will build a list of 3 ControlRunInstances)
	tree.PopulateControlRunInstances()

	err = displayControlResults(checkCtx, tree, initData.OutputFormatter)
	if err != nil {
		return err
	}
	return nil
}

func publishSnapshot(ctx context.Context, executionTree *controlexecute.ExecutionTree, shouldShare bool, shouldUpload bool) error {
	if error_helpers.IsContextCanceled(ctx) {
		return ctx.Err()
	}
	// if the share args are set, create a snapshot and share it
	if shouldShare || shouldUpload {
		statushooks.SetStatus(ctx, "Publishing snapshot")
		return controldisplay.PublishSnapshot(ctx, executionTree, shouldShare)
	}
	return nil
}

func getExecutionTrees(ctx context.Context, initData *controlinit.InitData, client *db_client.DbClient) ([]*namedExecutionTree, error) {
	var trees []*namedExecutionTree
	if error_helpers.IsContextCanceled(ctx) {
		return nil, ctx.Err()
	}

	if initData.ExportManager.HasNamedExport(viper.GetStringSlice(constants.ArgExport)) {
		// if there is a named export - combine targets into a single tree
		executionTree, err := controlexecute.NewExecutionTree(ctx, initData.Workspace, client, initData.ControlFilter, initData.Targets...)
		if err != nil {
			return nil, sperr.WrapWithMessage(err, "could not create merged execution tree")
		}
		name := fmt.Sprintf("check.%s", initData.Workspace.Mod.ShortName)
		trees = append(trees, newNamedExecutionTree(name, executionTree))
	} else {
		// otherwise return multiple trees
		for _, target := range initData.Targets {
			if error_helpers.IsContextCanceled(ctx) {
				return nil, ctx.Err()
			}
			executionTree, err := controlexecute.NewExecutionTree(ctx, initData.Workspace, client, initData.ControlFilter, target)
			if err != nil {
				return nil, sperr.WrapWithMessage(err, "could not create execution tree for %s", target)
			}

			trees = append(trees, newNamedExecutionTree(target.Name(), executionTree))
		}
	}

	return trees, ctx.Err()
}

// get the exit code for successful check run
func getExitCode(alarms int, errors int) int {
	// 1 or more control errors, return exitCode=2
	if errors > 0 {
		return constants.ExitCodeControlsError
	}
	// 1 or more controls in alarm, return exitCode=1
	if alarms > 0 {
		return constants.ExitCodeControlsAlarm
	}
	// no controls in alarm/error
	return constants.ExitCodeSuccessful
}

// create the context for the check run - add a control status renderer
func createCheckContext(ctx context.Context) (context.Context, context.CancelFunc) {
	var cancel context.CancelFunc
	// if a dashboard timeout was specified, use that
	if executionTimeout := viper.GetInt(constants.ArgBenchmarkTimeout); executionTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(executionTimeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)

	}
	ctx = controlstatus.AddControlHooksToContext(ctx, controlstatus.NewStatusControlHooks())
	return ctx, cancel
}

func validateCheckArgs(ctx context.Context) error {

	if err := localcmdconfig.ValidateSnapshotArgs(ctx); err != nil {
		return err
	}

	if viper.IsSet(constants.ArgSearchPath) && viper.IsSet(constants.ArgSearchPathPrefix) {
		return fmt.Errorf("only one of --search-path or --search-path-prefix may be set")
	}

	// only 1 character is allowed for '--separator'
	if len(viper.GetString(constants.ArgSeparator)) > 1 {
		return fmt.Errorf("'--%s' can be 1 character long at most", constants.ArgSeparator)
	}

	// only 1 of 'share' and 'snapshot' may be set
	if viper.GetBool(constants.ArgShare) && viper.GetBool(constants.ArgSnapshot) {
		return fmt.Errorf("only 1 of '--%s' and '--%s' may be set", constants.ArgShare, constants.ArgSnapshot)
	}

	// if both '--where' and '--tag' have been used, then it's an error
	if viper.IsSet(constants.ArgWhere) && viper.IsSet(constants.ArgTag) {
		return fmt.Errorf("only 1 of '--%s' and '--%s' may be set", constants.ArgWhere, constants.ArgTag)
	}

	return localcmdconfig.ValidateDatabaseArg()
}

func shouldPrintCheckTiming() bool {
	outputFormat := viper.GetString(constants.ArgOutput)

	return (viper.GetBool(constants.ArgTiming) && !viper.GetBool(constants.ArgDryRun)) &&
		(outputFormat == constants.OutputFormatText || outputFormat == constants.OutputFormatBrief)
}

func displayControlResults(ctx context.Context, executionTree *controlexecute.ExecutionTree, formatter controldisplay.Formatter) error {
	reader, err := formatter.Format(ctx, executionTree)
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, reader)
	return err
}

func displayDetectionResults(ctx context.Context, executionTree *dashboardexecute.DetectionBenchmarkDisplayTree, formatter controldisplay.Formatter) error {
	reader, err := formatter.FormatDetection(ctx, executionTree)
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, reader)
	return err
}

type namedExecutionTree struct {
	tree *controlexecute.ExecutionTree
	name string
}

func newNamedExecutionTree(name string, tree *controlexecute.ExecutionTree) *namedExecutionTree {
	return &namedExecutionTree{
		tree: tree,
		name: name,
	}
}
