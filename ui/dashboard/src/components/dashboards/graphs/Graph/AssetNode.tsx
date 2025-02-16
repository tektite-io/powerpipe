import DashboardIcon, {
  useDashboardIconType,
} from "@powerpipe/components/dashboards/common/DashboardIcon";
import Icon from "@powerpipe/components/Icon";
import IntegerDisplay from "@powerpipe/components/IntegerDisplay";
import RowProperties, { RowPropertiesTitle } from "./RowProperties";
import Tooltip from "./Tooltip";
import usePaginatedList from "@powerpipe/hooks/usePaginatedList";
import { buildLabelTextShadow } from "./utils";
import {
  Category,
  CategoryFold,
  CategoryProperties,
  FoldedNode,
  KeyValuePairs,
  KeyValueStringPairs,
} from "@powerpipe/components/dashboards/common/types";
import { classNames } from "@powerpipe/utils/styles";
import { ExpandedNodeInfo, useGraph } from "../common/useGraph";
import { getComponent } from "@powerpipe/components/dashboards";
import { Handle } from "reactflow";
import { injectSearchPathPrefix } from "@powerpipe/utils/url";
import { memo, ReactNode, useEffect, useMemo, useState } from "react";
import { useDashboardSearchPath } from "@powerpipe/hooks/useDashboardSearchPath";

type AssetNodeProps = {
  id: string;
  data: {
    category?: Category;
    color?: string;
    properties?: CategoryProperties;
    fold?: CategoryFold;
    href?: string;
    icon?: string;
    isFolded: boolean;
    foldedNodes?: FoldedNode[];
    label: string;
    row_data?: KeyValuePairs;
    themeColors: KeyValueStringPairs;
  };
};

type LabelProps = {
  children: ReactNode;
  themeColors: KeyValueStringPairs;
  title?: string;
};

type FoldedNodeCountBadgeProps = {
  foldedNodes: FoldedNode[] | undefined;
};

type FoldedNodeLabelProps = {
  category: Category | undefined;
  fold: CategoryFold | undefined;
  themeColors: KeyValueStringPairs;
};

type FoldedNodeTooltipTitleProps = {
  category: Category | undefined;
  foldedNodesCount: number;
};

type FolderNodeTooltipNodesProps = {
  foldedNodes: FoldedNode[] | undefined;
};

type NodeControlProps = {
  action?: () => void;
  className?: string;
  icon: string;
  iconClassName?: string;
  title?: string;
};

type NodeControlsProps = {
  children: ReactNode | ReactNode[];
};

type RefoldNodeControlProps = {
  collapseNodes: (foldedNodes: FoldedNode[]) => void;
  expandedNodeInfo: ExpandedNodeInfo | undefined;
};

const FoldedNodeTooltipTitle = ({
  category,
  foldedNodesCount,
}: FoldedNodeTooltipTitleProps) => (
  <div className="flex flex-col space-y-1">
    {category && (
      <span
        className="block text-foreground-lighter text-xs"
        style={{ color: category.color }}
      >
        {category.title || category.name}
      </span>
    )}
    <strong className="block">
      <IntegerDisplay num={foldedNodesCount} /> nodes
    </strong>
  </div>
);

const FoldedNodeTooltipNodes = ({
  foldedNodes,
}: FolderNodeTooltipNodesProps) => {
  const { visibleItems, hasMore, loadMore } = usePaginatedList(foldedNodes, 5);

  return (
    <div className="max-h-1/2-screen space-y-2">
      <div className="h-full overflow-y-auto">
        {(visibleItems || []).map((n) => (
          <div key={n.id}>{n.title || n.id}</div>
        ))}
        {hasMore && (
          <div
            className="flex items-center text-sm cursor-pointer space-x-1 text-link"
            onClick={loadMore}
          >
            <span>More</span>
            <Icon className="w-4 h-4" icon="south" />
          </div>
        )}
      </div>
    </div>
  );
};

const FoldedNodeCountBadge = ({ foldedNodes }: FoldedNodeCountBadgeProps) => {
  if (!foldedNodes) {
    return null;
  }
  return (
    <div className="absolute -right-[4%] -top-[4%] items-center bg-info text-white rounded-full px-1.5 text-sm font-medium cursor-pointer">
      <IntegerDisplay num={foldedNodes?.length || null} />
    </div>
  );
};

const Label = ({ children, themeColors, title }: LabelProps) => (
  <span
    style={{
      textShadow: buildLabelTextShadow(themeColors.dashboardPanel),
    }}
    title={title}
  >
    {children}
  </span>
);

const FoldedNodeLabel = ({
  category,
  fold,
  themeColors,
}: FoldedNodeLabelProps) => (
  <>
    {fold?.title && (
      <Label themeColors={themeColors} title={fold?.title}>
        {fold?.title}
      </Label>
    )}
    {!fold?.title && category?.title && (
      <Label themeColors={themeColors} title={category?.title}>
        {category?.title}
      </Label>
    )}
    {!fold?.title && !category?.title && (
      <Label themeColors={themeColors} title={category?.name}>
        {category?.name}
      </Label>
    )}
  </>
);

const NodeControl = ({
  action,
  className,
  icon,
  iconClassName,
  title,
}: NodeControlProps) => {
  return (
    <div
      onClick={(e) => {
        e.stopPropagation();
        action && action();
      }}
      className={classNames(className, "p-1")}
      title={title}
    >
      <Icon className={classNames(iconClassName, "w-3 h-3")} icon={icon} />
    </div>
  );
};

const NodeControls = ({ children }: NodeControlsProps) => {
  return (
    <div className="invisible group-hover:visible absolute -left-[17%] -bottom-[4%] flex flex-col space-y-px bg-dashboard text-foreground">
      {children}
    </div>
  );
};

const NodeGrabHandleControl = () => (
  <NodeControl
    className="custom-drag-handle cursor-grab"
    icon="zoom_out_map"
    iconClassName="rotate-45"
    title="Move node"
  />
);

const RefoldNodeControl = ({
  collapseNodes,
  expandedNodeInfo,
}: RefoldNodeControlProps) => {
  if (!expandedNodeInfo) {
    return null;
  }
  return (
    <NodeControl
      action={() => collapseNodes(expandedNodeInfo.foldedNodes)}
      className="cursor-pointer"
      icon="zoom_in_map"
      title="Collapse node"
    />
  );
};

const AssetNode = ({
  id,
  data: {
    category,
    color,
    properties,
    fold,
    icon,
    isFolded,
    foldedNodes,
    row_data,
    label,
    themeColors,
  },
}: AssetNodeProps) => {
  const { collapseNodes, expandNode, expandedNodes, renderResults } =
    useGraph();
  const { searchPathPrefix } = useDashboardSearchPath();
  const ExternalLink = getComponent("external_link");
  const iconType = useDashboardIconType(icon);
  const [renderedHref, setRenderedHref] = useState<string | null>(null);

  useEffect(() => {
    const renderResult = renderResults[id];
    if (!renderResult) {
      return;
    }
    if (!renderResult.result) {
      return;
    }
    const withSearchPathPrefix = injectSearchPathPrefix(
      renderResult.result,
      searchPathPrefix,
    );
    if (withSearchPathPrefix === renderedHref) {
      return;
    }
    setRenderedHref(withSearchPathPrefix);
  }, [id, renderedHref, renderResults, searchPathPrefix]);

  const isExpandedNode = useMemo(
    () => !!expandedNodes[id],
    [id, expandedNodes],
  );

  const textIconStringLength =
    iconType === "text" ? icon?.substring(5)?.length || 0 : null;

  const innerIcon = (
    <div
      className={classNames(
        iconType === "text" ? "p-1" : "p-3 leading-[50px]",
        "flex items-center justify-center rounded-full w-[50px] h-[50px] my-0 mx-auto border",
      )}
      style={{
        borderColor: color ? color : themeColors.blackScale3,
        color: isFolded ? (color ? color : themeColors.blackScale3) : undefined,
      }}
    >
      <DashboardIcon
        className={classNames(
          iconType === "text" ? "p-px overflow-hidden" : "max-w-full",
          // @ts-ignore
          iconType === "text" && textIconStringLength >= 6 ? "text-xs" : null,
          iconType === "text" && textIconStringLength === 5 ? "text-sm" : null,
          iconType === "text" &&
            // @ts-ignore
            textIconStringLength >= 3 &&
            // @ts-ignore
            textIconStringLength <= 4
            ? "text-lg"
            : null,
          // @ts-ignore
          iconType === "text" && textIconStringLength <= 2 ? "text-2xl" : null,
          iconType === "icon" && !color ? "text-foreground-lighter" : null,
        )}
        style={{
          color: color ? color : undefined,
        }}
        icon={isFolded ? fold?.icon : icon}
      />
      {isFolded && <FoldedNodeCountBadge foldedNodes={foldedNodes} />}
    </div>
  );

  const nodeIcon = (
    <div className="relative">
      {!renderedHref && innerIcon}
      {renderedHref && (
        <ExternalLink className="flex flex-col items-center" to={renderedHref}>
          {innerIcon}
        </ExternalLink>
      )}
      <NodeControls>
        {isExpandedNode && (
          <RefoldNodeControl
            collapseNodes={collapseNodes}
            expandedNodeInfo={expandedNodes[id]}
          />
        )}
        <NodeGrabHandleControl />
      </NodeControls>
    </div>
  );

  const innerNodeLabel = (
    <div
      className={classNames(
        renderedHref ? "text-link" : null,
        "absolute truncate bottom-0 px-1 text-sm mt-1 text-foreground whitespace-nowrap max-w-[150px]",
      )}
    >
      {!isFolded && (
        <Label themeColors={themeColors} title={label}>
          <>
            {label && label}
            {!label && category?.title && category.title}
            {!label && category?.name && category.name}
          </>
        </Label>
      )}
      {isFolded && (
        <FoldedNodeLabel
          category={category}
          fold={fold}
          themeColors={themeColors}
        />
      )}
    </div>
  );

  const nodeLabel = (
    <>
      {!renderedHref && innerNodeLabel}
      {renderedHref && (
        <ExternalLink className="flex flex-col items-center" to={renderedHref}>
          {innerNodeLabel}
        </ExternalLink>
      )}
    </>
  );

  const hasProperties = row_data && row_data.properties;

  const wrappedNode = (
    <div
      className={classNames(
        "group relative h-[72px] flex flex-col items-center",
        renderedHref || isFolded ? "cursor-pointer" : "cursor-auto",
      )}
      onClick={
        isFolded && foldedNodes
          ? () => expandNode(foldedNodes, category?.name as string)
          : undefined
      }
      title={isFolded ? "Expand nodes" : undefined}
    >
      {nodeIcon}
      {nodeLabel}
    </div>
  );

  // 4 possible node states
  // HREF  |  Folded  |  Properties  |  Controls
  // ----------------------------------------
  // false |  false   |  false       |  true
  // false |  true    |  true        |  true
  // true  |  false   |  false       |  true
  // true  |  false   |  true        |  true

  // Notes:
  // * The Handle elements seem to be required to allow the connectors to work.
  return (
    <>
      {/*@ts-ignore*/}
      <Handle className="hidden" isConnectable={false} type="target" />
      {/*@ts-ignore*/}
      <Handle className="hidden" isConnectable={false} type="source" />
      {!hasProperties && !isFolded && wrappedNode}
      {hasProperties && !isFolded && (
        <Tooltip
          overlay={
            <RowProperties
              propertySettings={properties || null}
              properties={row_data.properties}
            />
          }
          title={<RowPropertiesTitle category={category} title={label} />}
        >
          {wrappedNode}
        </Tooltip>
      )}
      {isFolded && (
        <Tooltip
          overlay={<FoldedNodeTooltipNodes foldedNodes={foldedNodes} />}
          title={
            <FoldedNodeTooltipTitle
              category={category}
              // @ts-ignore
              foldedNodesCount={foldedNodes.length}
            />
          }
        >
          {wrappedNode}
        </Tooltip>
      )}
    </>
  );
};

export default memo(AssetNode);
