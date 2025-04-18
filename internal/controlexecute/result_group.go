package controlexecute

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/pipe-fittings/v2/constants"
	"github.com/turbot/pipe-fittings/v2/error_helpers"
	"github.com/turbot/pipe-fittings/v2/modconfig"
	"github.com/turbot/pipe-fittings/v2/schema"
	"github.com/turbot/pipe-fittings/v2/steampipeconfig"
	"github.com/turbot/powerpipe/internal/controlstatus"
	"github.com/turbot/powerpipe/internal/db_client"
	"github.com/turbot/powerpipe/internal/resources"
	"golang.org/x/sync/semaphore"
)

const RootResultGroupName = "root_result_group"

// ResultGroup is a struct representing a grouping of control results
// It may correspond to a Benchmark, or some other arbitrary grouping
type ResultGroup struct {
	GroupId       string            `json:"name" csv:"group_id"`
	Title         string            `json:"title,omitempty" csv:"title"`
	Description   string            `json:"description,omitempty" csv:"description"`
	Tags          map[string]string `json:"tags,omitempty"`
	Documentation string            `json:"documentation,omitempty"`
	Display       string            `json:"display,omitempty"`
	Type          string            `json:"type,omitempty"`

	// the overall summary of the group
	Summary *GroupSummary `json:"summary"`
	// child result groups
	Groups []*ResultGroup `json:"-"`
	// child control runs
	ControlRuns []*ControlRun `json:"-"`
	// list of children stored as controlexecute.ExecutionTreeNode
	Children []ExecutionTreeNode                    `json:"-"`
	Severity map[string]controlstatus.StatusSummary `json:"-"`
	// "benchmark"
	NodeType string `json:"panel_type"`
	// the control tree item associated with this group(i.e. a mod/benchmark)
	GroupItem modconfig.ModTreeItem `json:"-"`
	Parent    *ResultGroup          `json:"-"`
	Duration  time.Duration         `json:"-"`

	// a list of distinct dimension keys from descendant controls
	DimensionKeys []string `json:"-"`

	childrenComplete   uint32
	executionStartTime time.Time
	// lock to prevent multiple control_runs updating this
	updateLock *sync.Mutex
}

type GroupSummary struct {
	Status   controlstatus.StatusSummary            `json:"status"`
	Severity map[string]controlstatus.StatusSummary `json:"-"`
}

func NewGroupSummary() *GroupSummary {
	return &GroupSummary{Severity: make(map[string]controlstatus.StatusSummary)}
}

// NewRootResultGroup creates a ResultGroup to act as the root node of a control execution tree
func NewRootResultGroup(ctx context.Context, executionTree *ExecutionTree, rootItem modconfig.ModTreeItem) (*ResultGroup, error) {
	root := &ResultGroup{
		GroupId:    RootResultGroupName,
		Groups:     []*ResultGroup{},
		Tags:       make(map[string]string),
		Summary:    NewGroupSummary(),
		Severity:   make(map[string]controlstatus.StatusSummary),
		updateLock: new(sync.Mutex),
		NodeType:   schema.BlockTypeBenchmark,
		Title:      rootItem.GetTitle(),
	}

	// if root item is a benchmark, create new result group with root as parent
	if control, ok := rootItem.(*resources.Control); ok {
		// if root item is a control, add control run
		if err := executionTree.AddControl(ctx, control, root); err != nil {
			return nil, err
		}
	} else {
		// create a result group for this item
		itemGroup, err := NewResultGroup(ctx, executionTree, rootItem, root)
		if err != nil {
			return nil, err
		}
		root.addResultGroup(itemGroup)
	}

	return root, nil
}

// NewResultGroup creates a result group from a ModTreeItem
func NewResultGroup(ctx context.Context, executionTree *ExecutionTree, treeItem modconfig.ModTreeItem, parent *ResultGroup) (*ResultGroup, error) {
	group := &ResultGroup{
		GroupId:     treeItem.Name(),
		Title:       treeItem.GetTitle(),
		Description: treeItem.GetDescription(),
		Tags:        treeItem.GetTags(),
		GroupItem:   treeItem,
		Parent:      parent,
		Groups:      []*ResultGroup{},
		Summary:     NewGroupSummary(),
		Severity:    make(map[string]controlstatus.StatusSummary),
		updateLock:  new(sync.Mutex),
		NodeType:    schema.BlockTypeBenchmark,
	}

	// populate additional properties (this avoids adding GetDocumentation, GetDisplay and GetType to all ModTreeItems)
	switch t := treeItem.(type) {
	case *resources.Benchmark:
		group.Documentation = t.GetDocumentation()
		group.Display = t.GetDisplay()
		group.Type = t.GetType()
	case *resources.Control:
		group.Documentation = t.GetDocumentation()
		group.Display = t.GetDisplay()
		group.Type = t.GetType()
	}
	// add child groups for children which are benchmarks
	for _, c := range treeItem.GetChildren() {
		if benchmark, ok := c.(*resources.Benchmark); ok {
			// create a result group for this item
			benchmarkGroup, err := NewResultGroup(ctx, executionTree, benchmark, group)
			if err != nil {
				return nil, err
			}
			// if the group has any control runs, add to tree
			if benchmarkGroup.ControlRunCount() > 0 {
				// create a new result group with 'group' as the parent
				group.addResultGroup(benchmarkGroup)
			}
		}
		if control, ok := c.(*resources.Control); ok {
			if err := executionTree.AddControl(ctx, control, group); err != nil {
				return nil, err
			}
		}
	}

	return group, nil
}

func (r *ResultGroup) AllTagKeys() []string {
	var tags []string
	for k := range r.Tags {
		tags = append(tags, k)
	}
	for _, child := range r.Groups {
		tags = append(tags, child.AllTagKeys()...)
	}
	for _, run := range r.ControlRuns {
		for k := range run.Control.Tags {
			tags = append(tags, k)
		}
	}
	tags = helpers.StringSliceDistinct(tags)
	sort.Strings(tags)
	return tags
}

// GetGroupByName finds an immediate child ResultGroup with a specific name
func (r *ResultGroup) GetGroupByName(name string) *ResultGroup {
	for _, group := range r.Groups {
		if group.GroupId == name {
			return group
		}
	}
	return nil
}

// GetChildGroupByName finds a nested child ResultGroup with a specific name
func (r *ResultGroup) GetChildGroupByName(name string) *ResultGroup {
	for _, group := range r.Groups {
		if group.GroupId == name {
			return group
		}
		if child := group.GetChildGroupByName(name); child != nil {
			return child
		}
	}
	return nil
}

// GetControlRunByName finds a child ControlRun with a specific control name
func (r *ResultGroup) GetControlRunByName(name string) *ControlRun {
	for _, run := range r.ControlRuns {
		if run.Control.Name() == name {
			return run
		}
	}
	return nil
}

func (r *ResultGroup) ControlRunCount() int {
	count := len(r.ControlRuns)
	for _, g := range r.Groups {
		count += g.ControlRunCount()
	}
	return count
}

// IsSnapshotPanel implements SnapshotPanel
func (*ResultGroup) IsSnapshotPanel() {}

// IsExecutionTreeNode implements ExecutionTreeNode
func (*ResultGroup) IsExecutionTreeNode() {}

// GetChildren implements ExecutionTreeNode
func (r *ResultGroup) GetChildren() []ExecutionTreeNode { return r.Children }

// GetName implements ExecutionTreeNode
func (r *ResultGroup) GetName() string { return r.GroupId }

// AsTreeNode implements ExecutionTreeNode
func (r *ResultGroup) AsTreeNode() *steampipeconfig.SnapshotTreeNode {
	res := &steampipeconfig.SnapshotTreeNode{
		Name:     r.GroupId,
		Children: make([]*steampipeconfig.SnapshotTreeNode, len(r.Children)),
		NodeType: r.NodeType,
	}
	for i, c := range r.Children {
		res.Children[i] = c.AsTreeNode()
	}
	return res
}

// add result group into our list, and also add a tree node into our child list
func (r *ResultGroup) addResultGroup(group *ResultGroup) {
	r.Groups = append(r.Groups, group)
	r.Children = append(r.Children, group)
}

// add control into our list, and also add a tree node into our child list
func (r *ResultGroup) addControl(controlRun *ControlRun) {
	r.ControlRuns = append(r.ControlRuns, controlRun)
	r.Children = append(r.Children, controlRun)
}

func (r *ResultGroup) addDimensionKeys(keys ...string) {
	r.updateLock.Lock()
	defer r.updateLock.Unlock()
	r.DimensionKeys = append(r.DimensionKeys, keys...)
	if r.Parent != nil {
		r.Parent.addDimensionKeys(keys...)
	}
	r.DimensionKeys = helpers.StringSliceDistinct(r.DimensionKeys)
	sort.Strings(r.DimensionKeys)
}

// onChildDone is a callback that gets called from the children of this result group when they are done
func (r *ResultGroup) onChildDone() {
	newCount := atomic.AddUint32(&r.childrenComplete, 1)
	totalCount := uint32(len(r.ControlRuns) + len(r.Groups)) //nolint:gosec // will not overflow
	if newCount < totalCount {
		// all children haven't finished execution yet
		return
	}

	// all children are done
	r.Duration = time.Since(r.executionStartTime)
	if r.Parent != nil {
		r.Parent.onChildDone()
	}
}

func (r *ResultGroup) updateSummary(summary *controlstatus.StatusSummary) {
	r.updateLock.Lock()
	defer r.updateLock.Unlock()

	r.Summary.Status.Skip += summary.Skip
	r.Summary.Status.Alarm += summary.Alarm
	r.Summary.Status.Info += summary.Info
	r.Summary.Status.Ok += summary.Ok
	r.Summary.Status.Error += summary.Error

	if r.Parent != nil {
		r.Parent.updateSummary(summary)
	}
}

func (r *ResultGroup) updateSeverityCounts(severity string, summary *controlstatus.StatusSummary) {
	r.updateLock.Lock()
	defer r.updateLock.Unlock()

	val, exists := r.Severity[severity]
	if !exists {
		val = controlstatus.StatusSummary{}
	}
	val.Alarm += summary.Alarm
	val.Error += summary.Error
	val.Info += summary.Info
	val.Ok += summary.Ok
	val.Skip += summary.Skip

	r.Summary.Severity[severity] = val
	if r.Parent != nil {
		r.Parent.updateSeverityCounts(severity, summary)
	}
}

func (r *ResultGroup) execute(ctx context.Context, client *db_client.DbClient, parallelismLock *semaphore.Weighted) {
	slog.Debug("begin ResultGroup.Execute", "group id", r.GroupId)
	defer slog.Debug("end ResultGroup.Execute", "group id", r.GroupId)

	r.executionStartTime = time.Now()

	for _, controlRun := range r.ControlRuns {
		if error_helpers.IsContextCanceled(ctx) {
			controlRun.setError(ctx, ctx.Err())
			continue
		}

		if viper.GetBool(constants.ArgDryRun) {
			controlRun.skip(ctx)
			continue
		}

		err := parallelismLock.Acquire(ctx, 1)
		if err != nil {
			controlRun.setError(ctx, err)
			continue
		}

		go executeRun(ctx, controlRun, parallelismLock, client)
	}
	for _, child := range r.Groups {
		child.execute(ctx, client, parallelismLock)
	}
}

func executeRun(ctx context.Context, run *ControlRun, parallelismLock *semaphore.Weighted, client *db_client.DbClient) {
	defer func() {
		if r := recover(); r != nil {
			// if the Execute panic'ed, set it as an error
			run.setError(ctx, helpers.ToError(r))
		}
		// Release in defer, so that we don't retain the lock even if there's a panic inside
		parallelismLock.Release(1)
	}()

	run.execute(ctx, client)
}
