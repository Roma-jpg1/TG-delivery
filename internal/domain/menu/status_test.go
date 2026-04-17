package menu

import "testing"

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status BranchItemStatus
		want   bool
	}{
		{name: "available", status: StatusAvailable, want: true},
		{name: "out_of_stock", status: StatusOutOfStock, want: true},
		{name: "disabled", status: StatusDisabled, want: true},
		{name: "hidden", status: StatusHidden, want: true},
		{name: "archived", status: StatusArchived, want: true},
		{name: "invalid", status: "sold_out", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidStatus(tt.status); got != tt.want {
				t.Fatalf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
