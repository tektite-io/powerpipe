import { createContext, ReactNode, useCallback, useContext } from "react";
import {
  DashboardDisplayMode,
  DashboardSearch,
  DashboardSearchGroupByMode,
} from "@powerpipe/types";
import { useSearchParams } from "react-router-dom";

interface IDashboardSearchContext {
  search: DashboardSearch;
  dashboardsDisplay: DashboardDisplayMode;
  updateDashboardsDisplay: (value: DashboardDisplayMode) => void;
  updateSearchValue: (value: string | undefined) => void;
  updateGroupBy: (value: DashboardSearchGroupByMode, tag?: string) => void;
}

interface DashboardSearchProviderProps {
  children: ReactNode;
  defaultSearch: DashboardSearch | undefined;
}

const DashboardSearchContext = createContext<IDashboardSearchContext | null>(
  null,
);

export const DashboardSearchProvider = ({
  children,
  defaultSearch,
}: DashboardSearchProviderProps) => {
  const [searchParams, setSearchParams] = useSearchParams();
  const search = {
    value: searchParams.get("search") || "",
    groupBy: {
      value:
        (searchParams.get("group_by") as DashboardSearchGroupByMode) ||
        defaultSearch?.groupBy?.value ||
        "tag",
      tag: searchParams.get("tag") || defaultSearch?.groupBy?.tag || "service",
    },
  };
  const dashboardsDisplay = (searchParams.get("dashboard_display") ||
    "top_level") as DashboardDisplayMode;

  const updateSearchValue = useCallback(
    (value: string | undefined) => {
      setSearchParams((previous) => {
        if (value) {
          previous.set("search", value);
        } else {
          previous.delete("search");
        }
        return previous;
      });
    },
    [setSearchParams],
  );

  const updateGroupBy = useCallback(
    (value: DashboardSearchGroupByMode, tag?: string) => {
      setSearchParams((previous) => {
        previous.set("group_by", value);
        if (tag) {
          previous.set("tag", tag);
        } else {
          previous.delete("tag");
        }
        return previous;
      });
    },
    [setSearchParams],
  );

  const updateDashboardsDisplay = useCallback(
    (value: DashboardDisplayMode) => {
      setSearchParams((previous) => {
        previous.set("dashboard_display", value);
        return previous;
      });
    },
    [setSearchParams],
  );

  return (
    <DashboardSearchContext.Provider
      value={{
        dashboardsDisplay,
        search,
        updateDashboardsDisplay,
        updateSearchValue,
        updateGroupBy,
      }}
    >
      {children}
    </DashboardSearchContext.Provider>
  );
};

export const useDashboardSearch = () => {
  const context = useContext(DashboardSearchContext);
  if (!context) {
    throw new Error(
      "useDashboardSearch must be used within a DashboardSearchContext",
    );
  }
  return context;
};
