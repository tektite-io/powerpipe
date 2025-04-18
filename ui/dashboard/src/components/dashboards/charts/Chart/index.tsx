import ErrorPanel from "@powerpipe/components/dashboards/Error";
import has from "lodash/has";
import isEmpty from "lodash/isEmpty";
import merge from "lodash/merge";
import Placeholder from "@powerpipe/components/dashboards/Placeholder";
import React, { useEffect, useRef, useState } from "react";
import ReactEChartsCore from "echarts-for-react/lib/core";
import set from "lodash/set";
import useChartThemeColors from "@powerpipe/hooks/useChartThemeColors";
import useMediaMode from "@powerpipe/hooks/useMediaMode";
import useTemplateRender from "@powerpipe/hooks/useTemplateRender";
import {
  buildChartDataset,
  getColorOverride,
  LeafNodeData,
  Width,
} from "@powerpipe/components/dashboards/common";
import { EChartsOption } from "echarts-for-react/src/types";
import {
  ChartProperties,
  ChartProps,
  ChartSeries,
  ChartSeriesOptions,
  ChartTransform,
  ChartType,
} from "@powerpipe/components/dashboards/charts/types";
import { FlowType } from "@powerpipe/components/dashboards/flows/types";
import { getChartComponent } from "@powerpipe/components/dashboards/charts";
import { GraphType } from "@powerpipe/components/dashboards/graphs/types";
import { HierarchyType } from "@powerpipe/components/dashboards/hierarchies/types";
import { injectSearchPathPrefix } from "@powerpipe/utils/url";
import { registerComponent } from "@powerpipe/components/dashboards";
import { useDashboardSearchPath } from "@powerpipe/hooks/useDashboardSearchPath";
import { useDashboardTheme } from "@powerpipe/hooks/useDashboardTheme";
import { useNavigate } from "react-router-dom";
import { parseDate } from "@powerpipe/utils/date";

const getThemeColorsWithPointOverrides = (
  type: ChartType = "column",
  series: any[],
  seriesOverrides: ChartSeries | undefined,
  dataset: any[][],
  themeColorValues,
) => {
  if (isEmpty(themeColorValues)) {
    return [];
  }
  switch (type) {
    case "donut":
    case "pie": {
      const newThemeColors: string[] = [];
      for (let rowIndex = 1; rowIndex < dataset.length; rowIndex++) {
        if (rowIndex - 1 < themeColorValues.charts.length) {
          newThemeColors.push(themeColorValues.charts[rowIndex - 1]);
        } else {
          newThemeColors.push(
            themeColorValues.charts[
              (rowIndex - 1) % themeColorValues.charts.length
            ],
          );
        }
      }
      series.forEach((seriesInfo) => {
        const seriesName = seriesInfo.name;
        const overrides = seriesOverrides
          ? seriesOverrides[seriesName] || {}
          : ({} as ChartSeriesOptions);
        const pointOverrides = overrides.points || {};
        dataset.slice(1).forEach((dataRow, dataRowIndex) => {
          const pointOverride = pointOverrides[dataRow[0]];
          if (pointOverride && pointOverride.color) {
            newThemeColors[dataRowIndex] = getColorOverride(
              pointOverride.color,
              themeColorValues,
            );
          }
        });
      });
      return newThemeColors;
    }
    default:
      const newThemeColors: string[] = [];
      for (let seriesIndex = 0; seriesIndex < series.length; seriesIndex++) {
        if (seriesIndex < themeColorValues.charts.length - 1) {
          newThemeColors.push(themeColorValues.charts[seriesIndex]);
        } else {
          newThemeColors.push(
            themeColorValues.charts[
              seriesIndex % themeColorValues.charts.length
            ],
          );
        }
      }
      return newThemeColors;
  }
};

const getCommonBaseOptions = (themeColors) => ({
  animation: false,
  grid: {
    left: "5%",
    right: "5%",
    top: "7%",
    bottom: "8%",
    // bottom: 40,
    containLabel: true,
  },
  legend: {
    orient: "horizontal",
    left: "center",
    top: "10",
    textStyle: {
      fontSize: 11,
      overflow: "truncate",
    },
  },
  textStyle: {
    fontFamily:
      'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"',
  },
  tooltip: {
    appendToBody: true,
    backgroundColor: themeColors.dashboard,
    borderColor: themeColors.dashboardPanel,
    borderWidth: 1,
    textStyle: {
      color: themeColors.foreground,
      fontSize: 11,
    },
    trigger: "item",
  },
});

const getXAxisLabelRotation = (number_of_rows: number) => {
  if (number_of_rows < 5) {
    return 0;
  }
  if (number_of_rows < 10) {
    return 30;
  }
  if (number_of_rows < 15) {
    return 45;
  }
  if (number_of_rows < 20) {
    return 60;
  }
  return 90;
};

const getXAxisLabelWidth = (number_of_rows: number) => {
  if (number_of_rows < 5) {
    return null;
  }
  if (number_of_rows < 10) {
    return 85;
  }
  if (number_of_rows < 15) {
    return 75;
  }
  if (number_of_rows < 20) {
    return 60;
  }
  return 50;
};

const getCommonBaseOptionsForChartType = (
  type: ChartType | undefined,
  width: Width | undefined,
  dataset: any[][],
  shouldBeTimeSeries: boolean,
  series: any[],
  seriesOverrides: ChartSeries | undefined,
  themeColors,
  dataConfig: any = {},
) => {
  switch (type) {
    case "heatmap": {
      return {
        grid: {
          top: "17%",
        },
        tooltip: {
          position: "top",
          formatter: function (params) {
            if (dataConfig.interval === "hourly") {
              return `${params.value[2].toLocaleString()} entries on ${params.value[0]} at ${params.value[1]}`;
            } else {
              return `${params.value[2].toLocaleString()} entries on ${params.value[0]}`;
            }
          },
        },
        visualMap: {
          type: "piecewise", // Use piecewise for custom range-color mapping
          show: true, // Display the legend at the top
          orient: "horizontal", // Horizontal layout for the legend
          top: 10, // Position the legend at the top
          left: "center",
          min: 0,
          max: dataConfig.maxValue,
          textStyle: {
            color: themeColors.foreground,
          },
          pieces: [
            { value: 0, color: themeColors.foregroundLightest, label: "0" },
            {
              min: 1,
              max: Math.floor(dataConfig.maxValue * 0.2) - 1,
              color: "#dae2fa",
              label: `1-${(Math.floor(dataConfig.maxValue * 0.2) - 1).toLocaleString()}`,
            },
            {
              min: Math.floor(dataConfig.maxValue * 0.2),
              max: Math.floor(dataConfig.maxValue * 0.4) - 1,
              color: "#b6c5f6",
              label: `${Math.floor(dataConfig.maxValue * 0.2).toLocaleString()}-${(Math.floor(dataConfig.maxValue * 0.4) - 1).toLocaleString()}`,
            },
            {
              min: Math.floor(dataConfig.maxValue * 0.4),
              max: Math.floor(dataConfig.maxValue * 0.6) - 1,
              color: "#91a7f1",
              label: `${Math.floor(dataConfig.maxValue * 0.4).toLocaleString()}-${(Math.floor(dataConfig.maxValue * 0.6) - 1).toLocaleString()}`,
            },
            {
              min: Math.floor(dataConfig.maxValue * 0.6),
              max: Math.floor(dataConfig.maxValue * 0.8) - 1,
              color: "#6d8aed",
              label: `${Math.floor(dataConfig.maxValue * 0.6).toLocaleString()}-${(Math.floor(dataConfig.maxValue * 0.8) - 1).toLocaleString()}`,
            },
            {
              min: Math.floor(dataConfig.maxValue * 0.8),
              max: dataConfig.maxValue,
              color: "#486de8",
              label: `${Math.floor(dataConfig.maxValue * 0.8).toLocaleString()}-${dataConfig.maxValue.toLocaleString()}`,
            },
          ],
        },
        xAxis: {
          type: "category",
          data: dataConfig.xAxisData,
          axisLabel: {
            color: themeColors.foreground,
            //fontSize: 10,
          },
        },
        yAxis: {
          type: "category",
          data: dataConfig.yAxisData,
          axisLabel: {
            color: themeColors.foreground,
            //fontSize: 10,
          },
        },
      };
    }
    case "bar":
      return {
        color: getThemeColorsWithPointOverrides(
          type,
          series,
          seriesOverrides,
          dataset,
          themeColors,
        ),
        legend: {
          show: series ? series.length > 1 : false,
          textStyle: {
            color: themeColors.foreground,
          },
        },
        // Declare an x-axis (category axis).
        // The category map the first row in the dataset by default.
        xAxis: {
          axisLabel: { color: themeColors.foreground, fontSize: 10 },
          axisLine: {
            show: true,
            lineStyle: { color: themeColors.foregroundLightest },
          },
          axisTick: { show: true },
          nameGap: 25,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
          splitLine: { show: false },
        },
        // Declare a y-axis (value axis).
        yAxis: {
          type: "category",
          axisLabel: {
            color: themeColors.foreground,
            overflow: "truncate",
          },
          axisLine: { lineStyle: { color: themeColors.foregroundLightest } },
          axisTick: { show: false },
          nameGap: 100,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
        },
      };
    case "area":
    case "line":
      return {
        color: getThemeColorsWithPointOverrides(
          type,
          series,
          seriesOverrides,
          dataset,
          themeColors,
        ),
        legend: {
          show: series ? series.length > 1 : false,
          textStyle: {
            color: themeColors.foreground,
          },
        },
        // Declare an x-axis (category or time axis, depending on the type of the first column).
        // The category/time map the first row in the dataset by default.
        xAxis: {
          type: shouldBeTimeSeries ? "time" : "category",
          boundaryGap: type !== "area",
          axisLabel: {
            color: themeColors.foreground,
            fontSize: 10,
            rotate: getXAxisLabelRotation(dataset.length - 1),
            width: getXAxisLabelWidth(dataset.length),
            overflow: "truncate",
          },
          axisLine: { lineStyle: { color: themeColors.foregroundLightest } },
          axisTick: { show: false },
          nameGap: 30,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
        },
        // Declare a y-axis (value axis).
        yAxis: {
          axisLabel: { color: themeColors.foreground, fontSize: 10 },
          axisLine: {
            show: true,
            lineStyle: { color: themeColors.foregroundLightest },
          },
          axisTick: { show: true },
          splitLine: { show: false },
          nameGap: width ? width + 42 : 50,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
        },
        tooltip: {
          trigger: "axis",
        },
      };
    case "column":
      return {
        color: getThemeColorsWithPointOverrides(
          type,
          series,
          seriesOverrides,
          dataset,
          themeColors,
        ),
        legend: {
          show: series ? series.length > 1 : false,
          textStyle: {
            color: themeColors.foreground,
          },
        },
        // Declare an x-axis (category or time axis, depending on the value of the first column).
        // The category/time map the first row in the dataset by default.
        xAxis: {
          type: shouldBeTimeSeries ? "time" : "category",
          axisLabel: {
            color: themeColors.foreground,
            fontSize: 10,
            rotate: getXAxisLabelRotation(dataset.length - 1),
            width: getXAxisLabelWidth(dataset.length),
            overflow: "truncate",
          },
          axisLine: { lineStyle: { color: themeColors.foregroundLightest } },
          axisTick: { show: false },
          nameGap: 30,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
        },
        // Declare a y-axis (value axis).
        yAxis: {
          axisLabel: { color: themeColors.foreground, fontSize: 10 },
          axisLine: {
            show: true,
            lineStyle: { color: themeColors.foregroundLightest },
          },
          axisTick: { show: true },
          splitLine: { show: false },
          nameGap: width ? width + 42 : 50,
          nameLocation: "center",
          nameTextStyle: { color: themeColors.foreground },
        },
        ...(shouldBeTimeSeries ? { tooltip: { trigger: "axis" } } : {}),
      };
    case "pie":
      return {
        color: getThemeColorsWithPointOverrides(
          type,
          series,
          seriesOverrides,
          dataset,
          themeColors,
        ),
        legend: {
          show: false,
          textStyle: {
            color: themeColors.foreground,
          },
        },
      };
    case "donut":
      return {
        color: getThemeColorsWithPointOverrides(
          type,
          series,
          seriesOverrides,
          dataset,
          themeColors,
        ),
        legend: {
          show: false,
          textStyle: {
            color: themeColors.foreground,
          },
        },
      };
    default:
      return {};
  }
};

const getOptionOverridesForChartType = (
  type: ChartType = "column",
  properties: ChartProperties | undefined,
  shouldBeTimeSeries: boolean,
) => {
  if (!properties) {
    return {};
  }

  let overrides = {};

  // orient: "horizontal",
  //     left: "center",
  //     top: "top",

  if (properties.legend) {
    // Legend display
    const legendDisplay = properties.legend.display;
    if (legendDisplay === "all") {
      overrides = set(overrides, "legend.show", true);
    } else if (legendDisplay === "none") {
      overrides = set(overrides, "legend.show", false);
    }

    // Legend display position
    const legendPosition = properties.legend.position;
    if (legendPosition === "top") {
      overrides = set(overrides, "legend.orient", "horizontal");
      overrides = set(overrides, "legend.left", "center");
      overrides = set(overrides, "legend.top", 10);
      overrides = set(overrides, "legend.bottom", "auto");
    } else if (legendPosition === "right") {
      overrides = set(overrides, "legend.orient", "vertical");
      overrides = set(overrides, "legend.left", 10);
      overrides = set(overrides, "legend.top", "middle");
      overrides = set(overrides, "legend.bottom", "auto");
    } else if (legendPosition === "bottom") {
      overrides = set(overrides, "legend.orient", "horizontal");
      overrides = set(overrides, "legend.left", "center");
      overrides = set(overrides, "legend.top", "auto");
      overrides = set(overrides, "legend.bottom", 10);
    } else if (legendPosition === "left") {
      overrides = set(overrides, "legend.orient", "vertical");
      overrides = set(overrides, "legend.left", 10);
      overrides = set(overrides, "legend.top", "middle");
      overrides = set(overrides, "legend.bottom", "auto");
    }
  }

  // Axes settings
  if (properties.axes) {
    // X axis settings
    if (properties.axes.x) {
      // X axis display setting
      const xAxisDisplay = properties.axes.x.display;
      if (xAxisDisplay === "all") {
        overrides = set(overrides, "xAxis.show", true);
      } else if (xAxisDisplay === "none") {
        overrides = set(overrides, "xAxis.show", false);
      }

      // X axis min setting
      if (type === "bar" && has(properties, "axes.x.min")) {
        overrides = set(overrides, "xAxis.min", properties.axes.x.min);
      }
      // Y axis max setting
      if (type === "bar" && has(properties, "axes.x.max")) {
        overrides = set(overrides, "xAxis.max", properties.axes.x.max);
      }

      // X axis labels settings
      if (properties.axes.x.labels) {
        // X axis labels display setting
        const xAxisTicksDisplay = properties.axes.x.labels.display;
        if (xAxisTicksDisplay === "all") {
          overrides = set(overrides, "xAxis.axisLabel.show", true);
        } else if (xAxisTicksDisplay === "none") {
          overrides = set(overrides, "xAxis.axisLabel.show", false);
        }
      }

      // X axis title settings
      if (properties.axes.x.title) {
        // X axis title display setting
        const xAxisTitleDisplay = properties.axes.x.title.display;
        if (xAxisTitleDisplay === "none") {
          overrides = set(overrides, "xAxis.name", null);
        }

        // X Axis title align setting
        const xAxisTitleAlign = properties.axes.x.title.align;
        if (xAxisTitleAlign === "start") {
          overrides = set(overrides, "xAxis.nameLocation", "start");
        } else if (xAxisTitleAlign === "center") {
          overrides = set(overrides, "xAxis.nameLocation", "center");
        } else if (xAxisTitleAlign === "end") {
          overrides = set(overrides, "xAxis.nameLocation", "end");
        }

        // X Axis title value setting
        const xAxisTitleValue = properties.axes.x.title.value;
        if (xAxisTitleValue) {
          overrides = set(overrides, "xAxis.name", xAxisTitleValue);
        }
      }

      // X Axis range setting (for timeseries plots)
      // Valid chart types: column, area, line (bar, donut and pie make no sense)
      if (["column", "area", "line"].includes(type) && shouldBeTimeSeries) {
        // X axis min setting (for timeseries)
        if (has(properties, "axes.x.min")) {
          // ECharts wants millis since epoch, not seconds
          overrides = set(overrides, "xAxis.min", properties.axes.x.min * 1000);
        }
        // Y axis max setting (for timeseries)
        if (has(properties, "axes.x.max")) {
          // ECharts wants millis since epoch, not seconds
          overrides = set(overrides, "xAxis.max", properties.axes.x.max * 1000);
        }
      }
    }

    // Y axis settings
    if (properties.axes.y) {
      // Y axis display setting
      const yAxisDisplay = properties.axes.y.display;
      if (yAxisDisplay === "all") {
        overrides = set(overrides, "yAxis.show", true);
      } else if (yAxisDisplay === "none") {
        overrides = set(overrides, "yAxis.show", false);
      }

      // Y axis min setting
      if (type !== "bar" && has(properties, "axes.y.min")) {
        overrides = set(overrides, "yAxis.min", properties.axes.y.min);
      }
      // Y axis max setting
      if (type !== "bar" && has(properties, "axes.y.max")) {
        overrides = set(overrides, "yAxis.max", properties.axes.y.max);
      }

      // Y axis labels settings
      if (properties.axes.y.labels) {
        // Y axis labels display setting
        const yAxisTicksDisplay = properties.axes.y.labels.display;
        if (yAxisTicksDisplay === "all") {
          overrides = set(overrides, "yAxis.axisLabel.show", true);
        } else if (yAxisTicksDisplay === "none") {
          overrides = set(overrides, "yAxis.axisLabel.show", false);
        }
      }

      // Y axis title settings
      if (properties.axes.y.title) {
        // Y axis title display setting
        const yAxisTitleDisplay = properties.axes.y.title.display;
        if (yAxisTitleDisplay === "none") {
          overrides = set(overrides, "yAxis.name", null);
        }

        // Y Axis title align setting
        const yAxisTitleAlign = properties.axes.y.title.align;
        if (yAxisTitleAlign === "start") {
          overrides = set(overrides, "yAxis.nameLocation", "start");
        } else if (yAxisTitleAlign === "center") {
          overrides = set(overrides, "yAxis.nameLocation", "center");
        } else if (yAxisTitleAlign === "end") {
          overrides = set(overrides, "yAxis.nameLocation", "end");
        }

        // Y Axis title value setting
        const yAxisTitleValue = properties.axes.y.title.value;
        if (yAxisTitleValue) {
          overrides = set(overrides, "yAxis.name", yAxisTitleValue);
        }
      }
    }
  }

  return overrides;
};

const getSeriesForChartType = (
  type: ChartType = "column",
  data: LeafNodeData | undefined,
  properties: ChartProperties | undefined,
  rowSeriesLabels: string[],
  transform: ChartTransform,
  shouldBeTimeSeries: boolean,
  themeColors,
  dataConfig: any = {},
) => {
  if (!data) {
    return [];
  }
  const series: any[] = [];
  const seriesNames =
    transform === "crosstab"
      ? rowSeriesLabels
      : data.columns.slice(1).map((col) => col.name);
  const seriesLength = seriesNames.length;
  for (let seriesIndex = 0; seriesIndex < seriesLength; seriesIndex++) {
    let seriesName = seriesNames[seriesIndex];
    let seriesColor = "auto";
    let seriesOverrides;
    if (properties) {
      if (properties.series && properties.series[seriesName]) {
        seriesOverrides = properties.series[seriesName];
      }
      if (seriesOverrides && seriesOverrides.title) {
        seriesName = seriesOverrides.title;
      }
      if (seriesOverrides && seriesOverrides.color) {
        seriesColor = getColorOverride(seriesOverrides.color, themeColors);
      }
    }

    switch (type) {
      case "heatmap": {
        series.push({
          type: "heatmap",
          data: dataConfig.heatmapData,
          emphasis: {
            itemStyle: {
              borderColor: themeColors.dashboardPanel,
              borderWidth: 1,
            },
          },
          itemStyle: {
            borderRadius: [3, 3, 3, 3],
            borderWidth: 2, // Add a larger border for padding effect
            borderColor: themeColors.dashboardPanel, // Border color matches the background
          },
        });
        break;
      }
      case "bar":
      case "column":
        series.push({
          name: seriesName,
          type: "bar",
          ...(properties && properties.grouping === "compare"
            ? {}
            : { stack: "total" }),
          itemStyle: {
            borderRadius:
              // Only round the last series and take into account bar vs chart e.g. orientation
              seriesIndex + 1 === seriesLength
                ? type === "bar"
                  ? [0, 5, 5, 0]
                  : [5, 5, 0, 0]
                : undefined,
            color: seriesColor,
            borderColor: themeColors.dashboardPanel,
            borderWidth: 1,
          },
          emphasis: {
            itemStyle: {
              borderRadius: [5, 5],
            },
          },
          barMaxWidth: 75,
          // Per https://stackoverflow.com/a/56116442, when using time series you have to manually encode each series
          // We assume that the first dimension/column is the timestamp
          ...(shouldBeTimeSeries ? { encode: { x: 0, y: seriesName } } : {}),
          // label: {
          //   show: true,
          //   position: 'outside'
          // },
        });
        break;
      case "donut":
        series.push({
          name: seriesName,
          type: "pie",
          center: ["50%", "50%"],
          radius: ["30%", "50%"],
          label: { color: themeColors.foreground, fontSize: 10 },
          itemStyle: {
            borderRadius: 5,
            borderColor: themeColors.dashboardPanel,
            borderWidth: 2,
          },
          emphasis: {
            itemStyle: {
              color: "inherit",
            },
          },
        });
        break;
      case "pie":
        series.push({
          name: seriesName,
          type: "pie",
          center: ["50%", "40%"],
          radius: "50%",
          label: { color: themeColors.foreground, fontSize: 10 },
          emphasis: {
            itemStyle: {
              color: "inherit",
              shadowBlur: 5,
              shadowOffsetX: 0,
              shadowColor: "rgba(0, 0, 0, 0.5)",
            },
          },
          itemStyle: {
            borderRadius: 5,
            borderColor: themeColors.dashboardPanel,
            borderWidth: 2,
          },
        });
        break;
      case "area":
        series.push({
          name: seriesName,
          type: "line",
          ...(properties && properties.grouping === "compare"
            ? {}
            : { stack: "total" }),
          // Per https://stackoverflow.com/a/56116442, when using time series you have to manually encode each series
          // We assume that the first dimension/column is the timestamp
          ...(shouldBeTimeSeries ? { encode: { x: 0, y: seriesName } } : {}),
          areaStyle: {},
          emphasis: {
            focus: "series",
          },
          itemStyle: { color: seriesColor },
        });
        break;
      case "line":
        series.push({
          name: seriesName,
          type: "line",
          itemStyle: { color: seriesColor },
          // Per https://stackoverflow.com/a/56116442, when using time series you have to manually encode each series
          // We assume that the first dimension/column is the timestamp
          ...(shouldBeTimeSeries ? { encode: { x: 0, y: seriesName } } : {}),
        });
        break;
    }
  }
  return series;
};

const adjustGridConfig = (
  config: EChartsOption,
  properties: ChartProperties | undefined,
) => {
  let newConfig = { ...config };
  if (!!newConfig?.xAxis?.name) {
    newConfig = set(newConfig, "grid.containLabel", false);
    newConfig = set(newConfig, "grid.bottom", "20%");
  }
  if (!!newConfig?.yAxis?.name) {
    newConfig = set(newConfig, "grid.containLabel", false);
    newConfig = set(newConfig, "grid.left", "25%");
    newConfig = set(newConfig, "grid.bottom", "25%");
  }
  if (newConfig?.legend?.show) {
    const configuredPosition = properties?.legend?.position || "top";
    switch (configuredPosition) {
      case "top":
        newConfig = set(newConfig, "grid.top", "20%");
        newConfig = set(newConfig, "grid.top", "20%");
        break;
      case "right":
        newConfig = set(newConfig, "grid.right", "35%");
        break;
      case "bottom":
        newConfig = set(newConfig, "grid.bottom", "25%");
        break;
      case "left":
        newConfig = set(newConfig, "grid.left", "50%");
        break;
    }
  }
  return newConfig;
};

const getDataConfigForChartType = (
  type: ChartType = "column",
  dataset: any[][],
) => {
  switch (type) {
    case "heatmap": {
      const rawData = dataset.slice(1);

      // Infer the interval
      const timestamps = rawData.map((d) => parseDate(d[0])?.unix());
      const differences = timestamps.slice(1).map((t, i) => t - timestamps[i]);
      const avgDifference =
        differences.reduce((a, b) => a + b, 0) / differences.length;
      const interval = avgDifference <= 3600 ? "hourly" : "daily"; // 3600000 ms = 1 hour

      // Generate x and y axes based on the inferred interval
      let xAxisData,
        yAxisData,
        heatmapData,
        maxValue = 0;

      if (interval === "hourly") {
        xAxisData = Array.from(new Set(rawData.map((d) => d[0].split("T")[0]))); // Unique days
        yAxisData = Array.from(
          { length: 24 },
          (_, i) => `${i < 10 ? `0${i}` : i}:00`,
        ); // Hours
        heatmapData = rawData.map((d) => {
          const [date, time] = d[0].split("T");
          if (!maxValue || d[1] > maxValue) {
            maxValue = d[1];
          }
          return [date, time.split(":")[0] + ":00", d[1]];
        });
      } else {
        xAxisData = Array.from(new Set(rawData.map((d) => d[0].split("T")[0]))); // Unique days
        yAxisData = ["Daily"];
        heatmapData = rawData.map((d) => {
          const date = d[0].split("T")[0];
          if (!maxValue || d[1] > maxValue) {
            maxValue = d[1];
          }
          return [date, "Daily", d[1]];
        });
      }

      return { interval, heatmapData, xAxisData, yAxisData, maxValue };
    }
    default:
      return {};
  }
};

const buildChartOptions = (props: ChartProps, themeColors: any) => {
  const { dataset, rowSeriesLabels, transform } = buildChartDataset(
    props.data,
    props.properties,
  );
  const treatAsTimeSeries = ["timestamp", "timestamptz", "date"].includes(
    props.data?.columns[0].data_type.toLowerCase() || "",
  );
  const dataConfig = getDataConfigForChartType(props.display_type, dataset);
  const series = getSeriesForChartType(
    props.display_type || "column",
    props.data,
    props.properties,
    rowSeriesLabels,
    transform,
    treatAsTimeSeries,
    themeColors,
    dataConfig,
  );
  const config = merge(
    getCommonBaseOptions(themeColors),
    getCommonBaseOptionsForChartType(
      props.display_type || "column",
      props.width,
      dataset,
      treatAsTimeSeries,
      series,
      props.properties?.series,
      themeColors,
      dataConfig,
    ),
    getOptionOverridesForChartType(
      props.display_type || "column",
      props.properties,
      treatAsTimeSeries,
    ),
    { series },
    {
      dataset: {
        source: dataset,
      },
    },
  );
  return adjustGridConfig(config, props.properties);
};

type ChartComponentProps = {
  options: EChartsOption;
  searchPathPrefix: string[];
  type: ChartType | FlowType | GraphType | HierarchyType;
};

const handleClick = async (
  params: any,
  navigate,
  renderTemplates,
  searchPathPrefix,
) => {
  const componentType = params.componentType;
  if (componentType !== "series") {
    return;
  }
  const dataType = params.dataType;

  switch (dataType) {
    case "node":
      if (!params.data.href) {
        return;
      }
      const renderedResults = await renderTemplates(
        { graph_node: params.data.href as string },
        [params.data],
      );
      let rowRenderResult = renderedResults[0];
      const withSearchPathPrefix = injectSearchPathPrefix(
        rowRenderResult.graph_node.result,
        searchPathPrefix,
      );
      navigate(withSearchPathPrefix);
  }
};

const Chart = ({ options, searchPathPrefix, type }: ChartComponentProps) => {
  const [echarts, setEcharts] = useState<any | null>(null);
  const navigate = useNavigate();
  const chartRef = useRef<ReactEChartsCore>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const mediaMode = useMediaMode();
  const { ready: templateRenderReady, renderTemplates } = useTemplateRender();

  // Dynamically import echarts from its own bundle
  useEffect(() => {
    import("./echarts").then((m) => setEcharts(m.echarts));
  }, []);

  useEffect(() => {
    if (!chartRef.current || !options) {
      return;
    }

    const echartInstance = chartRef.current.getEchartsInstance();
    const dataURL = echartInstance.getDataURL({});
    if (dataURL === imageUrl) {
      return;
    }
    setImageUrl(dataURL);
  }, [chartRef, imageUrl, options]);

  if (!options) {
    return null;
  }

  const eventsDict = {
    click: (params) =>
      handleClick(params, navigate, renderTemplates, searchPathPrefix),
  };

  const PlaceholderComponent = Placeholder.component;

  return (
    <PlaceholderComponent ready={!!echarts && templateRenderReady}>
      <>
        {mediaMode !== "print" && (
          <div className="relative">
            <ReactEChartsCore
              ref={chartRef}
              echarts={echarts}
              className="chart-canvas"
              onEvents={eventsDict}
              option={options}
              notMerge={true}
              lazyUpdate={true}
              style={
                type === "pie" || type === "donut" ? { height: "250px" } : {}
              }
            />
          </div>
        )}
        {mediaMode === "print" && imageUrl && (
          <div>
            <img alt="Chart" className="max-w-full max-h-full" src={imageUrl} />
          </div>
        )}
      </>
    </PlaceholderComponent>
  );
};

const ChartWrapper = (props: ChartProps) => {
  const { wrapperRef } = useDashboardTheme();
  const { searchPathPrefix } = useDashboardSearchPath();
  const themeColors = useChartThemeColors();

  if (!wrapperRef) {
    return null;
  }

  if (!props.data) {
    return null;
  }

  return (
    <Chart
      options={buildChartOptions(props, themeColors)}
      searchPathPrefix={searchPathPrefix}
      type={props.display_type || "column"}
    />
  );
};

const renderChart = (definition: ChartProps) => {
  // We default to column charts if not specified
  const { display_type = "column" } = definition;

  const chart = getChartComponent(display_type);

  if (!chart) {
    return <ErrorPanel error={`Unknown chart type ${display_type}`} />;
  }

  const Component = chart.component;
  return <Component {...definition} />;
};

const RenderChart = (props: ChartProps) => {
  return renderChart(props);
};

registerComponent("chart", RenderChart);

export default ChartWrapper;

export { Chart };
