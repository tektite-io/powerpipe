package resources

import "github.com/turbot/pipe-fittings/v2/utils"

type DashboardChartSeriesPoint struct {
	Name  string  `hcl:"name,label" json:"-"`
	Color *string `cty:"color" hcl:"color" json:"color,omitempty"`
}

func (s DashboardChartSeriesPoint) Equals(other *DashboardChartSeriesPoint) bool {
	if other == nil {
		return false
	}

	return utils.SafeStringsEqual(s.Name, other.Name) &&
		utils.SafeStringsEqual(s.Color, other.Color)
}
