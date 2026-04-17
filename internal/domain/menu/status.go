package menu

type BranchItemStatus string

const (
	StatusAvailable  BranchItemStatus = "available"
	StatusOutOfStock BranchItemStatus = "out_of_stock"
	StatusDisabled   BranchItemStatus = "disabled"
	StatusHidden     BranchItemStatus = "hidden"
	StatusArchived   BranchItemStatus = "archived"
)

var validStatuses = map[BranchItemStatus]struct{}{
	StatusAvailable:  {},
	StatusOutOfStock: {},
	StatusDisabled:   {},
	StatusHidden:     {},
	StatusArchived:   {},
}

func IsValidStatus(status BranchItemStatus) bool {
	_, ok := validStatuses[status]
	return ok
}
