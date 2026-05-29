package picker

import (
	"strings"
	"testing"
)

func TestListViewFollowSelectionKeepsFocusedRowVisibleAtBottom(t *testing.T) {
	view := listView{
		Width:  80,
		Height: 3,
		Rows: []listRow{
			{Text: "row-01"},
			{Text: "row-02"},
			{Text: "row-03"},
			{Text: "row-04", Selected: true},
		},
	}

	got := view.Render()
	if strings.Contains(got, "row-01") {
		t.Fatalf("did not expect first row in bottom viewport, got %q", got)
	}
	if !strings.Contains(got, "row-04") {
		t.Fatalf("expected selected row in viewport, got %q", got)
	}
}

func TestListViewCenterSelectionUsesFocusedRowBeforeSelectedRows(t *testing.T) {
	view := listView{
		Width:        80,
		Height:       3,
		ViewportMode: listViewportCenterSelection,
		Rows: []listRow{
			{Text: "row-01", Selected: true},
			{Text: "row-02"},
			{Text: "row-03"},
			{Text: "row-04"},
			{Text: "row-05", Focused: true},
			{Text: "row-06"},
			{Text: "row-07"},
		},
	}

	got := view.Render()
	if strings.Contains(got, "row-01") {
		t.Fatalf("did not expect selected but unfocused row in centered viewport, got %q", got)
	}
	if !strings.Contains(got, "row-05") {
		t.Fatalf("expected focused row in viewport, got %q", got)
	}
}
