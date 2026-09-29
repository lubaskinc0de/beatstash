package library

import "slices"

// NavidromeAccess is a value object: the Navidrome libraries an account sees.
type NavidromeAccess struct {
	Admin      bool
	LibraryIDs []int
}

// Visible picks the Attached Libraries the account sees.
func (a NavidromeAccess) Visible(attached []Library) []*Library {
	var visible []*Library
	for n := range attached {
		lib := &attached[n]
		if a.Admin || slices.Contains(a.LibraryIDs, lib.NavidromeID) {
			visible = append(visible, lib)
		}
	}
	return visible
}

// KeptLibraries are where the user keeps music: their Personal Library and
// the Attached Libraries they see.
func KeptLibraries(personal *Library, visible []*Library) []*Library {
	return append([]*Library{personal}, visible...)
}
