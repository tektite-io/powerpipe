package dashboardexecute

import (
	"context"
	"fmt"
	"github.com/turbot/powerpipe/internal/controlexecute"
	"log/slog"
	"time"

	"github.com/turbot/pipe-fittings/v2/backend"
	"github.com/turbot/pipe-fittings/v2/connection"
	"github.com/turbot/pipe-fittings/v2/statushooks"
	"github.com/turbot/pipe-fittings/v2/steampipeconfig"
	"github.com/turbot/powerpipe/internal/dashboardtypes"
	"github.com/turbot/powerpipe/internal/db_client"
	"github.com/turbot/powerpipe/internal/resources"
	"github.com/turbot/powerpipe/internal/snapshot"
)

// DetectionRun is a struct representing the execution of a leaf dashboard node
type DetectionRun struct {
	// all RuntimeDependencySubscribers are also publishers as they have args/params
	RuntimeDependencySubscriberImpl

	Resource *resources.Detection `json:"-"`
	// this is populated by retrieving Resource properties with the snapshot tag
	Properties map[string]any           `json:"properties,omitempty"`
	Data       *dashboardtypes.LeafData `json:"data,omitempty"`
	// function called when the run is complete
	// this property populated for 'with' runs
	onComplete       func()
	database         connection.ConnectionStringProvider
	searchPathConfig backend.SearchPathConfig
}

func (r *DetectionRun) IsExecutionTreeNode() {
}

func (r *DetectionRun) GetChildren() []controlexecute.ExecutionTreeNode {
	return nil
}

func (r *DetectionRun) AsTreeNode() *steampipeconfig.SnapshotTreeNode {
	return &steampipeconfig.SnapshotTreeNode{
		Name:     r.Name,
		NodeType: r.NodeType,
	}
}

func NewDetectionRun(resource *resources.Detection, parent dashboardtypes.DashboardParent, executionTree *DashboardExecutionTree) (*DetectionRun, error) {
	r := &DetectionRun{
		Resource:   resource,
		Properties: make(map[string]any),
	}

	// create RuntimeDependencySubscriberImpl- this handles 'with' run creation and resolving runtime dependency resolution
	// (NOTE: we have to do this after creating run as we need to pass a ref to the run)
	r.RuntimeDependencySubscriberImpl = *NewRuntimeDependencySubscriber(resource, parent, r, executionTree)

	// now initialise database and search path
	if err := r.resolveDatabaseConfig(); err != nil {
		return nil, err
	}

	err := r.initRuntimeDependencies(executionTree)
	if err != nil {
		return nil, err
	}

	r.NodeType = resource.GetBlockType()

	// if the node has no runtime dependencies, resolve the sql
	if !r.hasRuntimeDependencies() {
		if err := r.resolveSQLAndArgs(); err != nil {
			return nil, err
		}
	}
	// add r into execution tree
	executionTree.runs[r.Name] = r

	// create buffered channel for children to report their completion
	r.createChildCompleteChan()

	// populate the names of any withs we depend on
	r.setRuntimeDependencies()

	if err := r.populateProperties(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *DetectionRun) resolveDatabaseConfig() error {
	// resolve the database and connection string for the run
	database, searchPathConfig, err := db_client.GetDatabaseConfigForResource(r.Resource, r.executionTree.workspace.Mod, r.executionTree.database, r.executionTree.searchPathConfig)
	if err != nil {
		return err
	}

	r.database = database
	r.searchPathConfig = searchPathConfig
	return nil
}

// Execute implements DashboardTreeRun
func (r *DetectionRun) Execute(ctx context.Context) {
	defer func() {
		// call our oncomplete is we have one
		// (this is used to collect 'with' data and propagate errors)
		if r.onComplete != nil {
			r.onComplete()
		}
	}()

	// if there is nothing to do, return
	if r.Status.IsFinished() {
		return
	}

	slog.Debug("DetectionRun Execute()", "name", r.resource.Name())

	// to get here, we must be a query provider

	// if we have children and with runs, start them asynchronously (they may block waiting for our runtime dependencies)
	r.executeChildrenAsync(ctx)

	// start a goroutine to wait for children to complete
	doneChan := r.waitForChildrenAsync(ctx)

	if err := r.evaluateRuntimeDependencies(ctx); err != nil {
		r.SetError(ctx, err)
		return
	}

	// set status to running (this sends update event)
	// (if we have blocked children, this will be changed to blocked)
	r.setRunning(ctx)

	// if we have sql to execute, do it now
	// (if we are only performing a base execution, do not run the query)
	if r.executeSQL != "" {
		if err := r.executeQuery(ctx); err != nil {
			r.SetError(ctx, err)
			return
		}
	}

	// wait for all children and withs
	err := <-doneChan
	if err == nil {
		slog.Debug("children complete", "name", r.resource.Name())

		// aggregate our child data
		//r.combineChildData()
		// set complete status on dashboard
		r.SetComplete(ctx)
	} else {
		slog.Debug("children complete with error", "name", r.resource.Name(), "error", err.Error())
		r.SetError(ctx, err)
	}
}

// SetError implements DashboardTreeRun (override to set snapshothook status)
func (r *DetectionRun) SetError(ctx context.Context, err error) {
	// increment error count for snapshot hook
	statushooks.SnapshotError(ctx)
	r.DashboardTreeRunImpl.SetError(ctx, err)
}

// SetComplete implements DashboardTreeRun (override to set snapshothook status
func (r *DetectionRun) SetComplete(ctx context.Context) {
	// call snapshot hooks with progress
	statushooks.UpdateSnapshotProgress(ctx, 1)

	r.DashboardTreeRunImpl.SetComplete(ctx)
}

// IsSnapshotPanel implements SnapshotPanel
func (*DetectionRun) IsSnapshotPanel() {}

// if this leaf run has a query or sql, execute it now
func (r *DetectionRun) executeQuery(ctx context.Context) error {
	slog.Debug("DetectionRun SQL resolved, executing", "name", r.resource.Name())

	// check for context errors
	if err := ctx.Err(); err != nil {
		if err.Error() == context.DeadlineExceeded.Error() {
			err = fmt.Errorf("dashboard execution timed out before execution of this node started")
		}
		return err
	}

	// get the client for this leaf run
	// (we have already resolved the database and search path config)
	client, err := r.executionTree.getClient(ctx, r.database, r.searchPathConfig)
	if err != nil {
		return err
	}

	startTime := time.Now()
	queryResult, err := client.ExecuteSync(ctx, r.executeSQL, r.Args...)
	if err != nil {
		if err.Error() == context.DeadlineExceeded.Error() {
			err = fmt.Errorf("query execution timed out after running for %0.2fs", time.Since(startTime).Seconds())
		}
		slog.Warn("DetectionRun query failed", "name", r.resource.Name(), "error", err.Error())
		return err
	}
	slog.Debug("DetectionRun complete", "name", r.resource.Name())

	r.Data, err = dashboardtypes.NewLeafData(queryResult)
	if err != nil {
		return err

	}
	return nil
}

//
//func (r *DetectionRun) combineChildData() {
//	// we either have children OR a query
//	// if there are no children, do nothing
//	if len(r.children) == 0 {
//		return
//	}
//	// create empty data to populate
//	r.Data = &dashboardtypes.LeafData{}
//	// build map of columns for the schema
//	schemaMap := make(map[string]*queryresult.ColumnDef)
//	for _, c := range r.children {
//		childDetectionRun := c.(*DetectionRun)
//		data := childDetectionRun.Data
//		// if there is no data or this is a 'with', skip
//		if data == nil || childDetectionRun.resource.GetBlockType() == schema.BlockTypeWith {
//			continue
//		}
//		for _, s := range data.Columns {
//			if _, ok := schemaMap[s.Name]; !ok {
//				schemaMap[s.Name] = s
//			}
//		}
//		r.Data.Rows = append(r.Data.Rows, data.Rows...)
//	}
//	r.Data.Columns = maps.Values(schemaMap)
//}

func (r *DetectionRun) populateProperties() error {
	if r.resource == nil {
		return nil
	}
	properties, err := snapshot.GetAsSnapshotPropertyMap(r.resource)
	if err != nil {
		return err

	}
	r.Properties = properties
	return nil
}
