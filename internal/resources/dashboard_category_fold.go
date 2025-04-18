package resources

import (
	"github.com/turbot/pipe-fittings/v2/utils"
)

type DashboardCategoryFold struct {
	Title     *string `cty:"title" hcl:"title" snapshot:"title" json:"title,omitempty"`
	Threshold *int    `cty:"threshold" hcl:"threshold" snapshot:"threshold" json:"threshold,omitempty"`
	Icon      *string `cty:"icon" hcl:"icon" snapshot:"icon" json:"icon,omitempty"`
}

func (f DashboardCategoryFold) Equals(other *DashboardCategoryFold) bool {
	if other == nil {
		return false
	}

	return utils.SafeStringsEqual(f.Title, other.Title) &&
		f.Threshold == other.Threshold &&
		utils.SafeStringsEqual(f.Icon, other.Icon)
}
