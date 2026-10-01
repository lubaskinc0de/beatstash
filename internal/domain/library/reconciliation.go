// Reconciliation: the plan that makes a Library's Tracks follow its files
// changed by hand. LibraryFile is a file as the bot found it on disk; a
// Track records its size, modification time and inode, so the next
// Reconciliation can tell whether the file changed or moved. Strays are
// files put into the Shared Library by hand: a Track gets there only
// through a Share.

package library

import (
	"slices"
	"time"
)

// LibraryFile is a value object: an audio file as the bot found it in a
// Library. Path is relative to the Library's Dir. A hardlink shares the
// Inode of its file.
type LibraryFile struct {
	Path    string
	Size    int64
	ModTime time.Time
	Inode   uint64
}

// Survey is a value object: the paths where a Library's Tracks and files
// disagreed at one moment, and the files whose audio has to be read. The
// disagreement may be gone by the time it is settled, so Reconcile judges
// these paths again, and only these.
type Survey struct {
	paths  []string
	unread []LibraryFile
}

// Reading is a value object: what a file's audio said when it was read.
type Reading struct {
	File  LibraryFile
	Probe Probe
}

// Reconciliation is a value object: the changes that bring a Library's
// Tracks in step with its files. The files stay as they are.
type Reconciliation struct {
	Save   []*Track
	Delete []*Track
	Strays []LibraryFile
}

type differences struct {
	changed []fileOf
	moved   []fileOf
	gone    []*Track
	added   []LibraryFile
}

type fileOf struct {
	track *Track
	file  LibraryFile
}

// SurveyFiles compares the Tracks of a Library with its files. It ignores
// files of no audio format the bot knows.
func SurveyFiles(tracks []Track, files []LibraryFile) Survey {
	d := compare(tracks, files)
	var s Survey
	for _, c := range d.changed {
		s.paths = append(s.paths, c.file.Path)
		s.unread = append(s.unread, c.file)
	}
	for _, m := range d.moved {
		s.paths = append(s.paths, m.track.Path, m.file.Path)
		if !m.track.Holds(m.file) {
			s.unread = append(s.unread, m.file)
		}
	}
	for _, track := range d.gone {
		s.paths = append(s.paths, track.Path)
	}
	for _, file := range d.added {
		s.paths = append(s.paths, file.Path)
		s.unread = append(s.unread, file)
	}
	return s
}

func (s Survey) Empty() bool {
	return len(s.paths) == 0
}

// Paths are the paths Reconcile judges.
func (s Survey) Paths() []string {
	return s.paths
}

// Unread are the files whose audio Reconcile needs.
func (s Survey) Unread() []LibraryFile {
	return s.unread
}

// Reconcile plans the changes for the surveyed paths from the Tracks and
// the files at those paths as they are now. A changed file is followed
// only if its reading is of the file as it is now; otherwise the Track
// waits for the next Reconciliation.
func (s Survey) Reconcile(lib *Library, tracks []Track, files []LibraryFile, readings []Reading) Reconciliation {
	read := make(map[string]Reading, len(readings))
	for _, reading := range readings {
		read[reading.File.Path] = reading
	}
	// probe is false unless the file was read as it is now.
	probe := func(file LibraryFile) (Probe, bool) {
		reading, ok := read[file.Path]
		return reading.Probe, ok && reading.File.holds(file.Size, file.ModTime)
	}

	var r Reconciliation
	d := compare(tracks, files)
	for _, c := range d.changed {
		if p, ok := probe(c.file); ok {
			c.track.FollowFile(c.file, p)
			r.Save = append(r.Save, c.track)
		}
	}
	for _, m := range d.moved {
		m.track.MoveTo(m.file.Path)
		if p, ok := probe(m.file); ok && !m.track.Holds(m.file) {
			m.track.FollowFile(m.file, p)
		}
		r.Save = append(r.Save, m.track)
	}
	r.Delete = d.gone
	for _, file := range d.added {
		if lib.Kind == LibraryShared {
			r.Strays = append(r.Strays, file)
			continue
		}
		if p, ok := probe(file); ok {
			r.Save = append(r.Save, NewFoundTrack(lib, file, p))
		}
	}
	return r
}

func (r Reconciliation) Empty() bool {
	return len(r.Save) == 0 && len(r.Delete) == 0
}

// Format is false for a file of no audio format the bot knows.
func (f LibraryFile) Format() (Format, bool) {
	return FormatOf(f.Path, "")
}

// holds tells whether the file is still as it was when it had the size and
// the modification time. Times count to the microsecond: a Track keeps no
// finer one.
func (f LibraryFile) holds(size int64, modTime time.Time) bool {
	return f.Size == size && f.ModTime.Truncate(time.Microsecond).Equal(modTime.Truncate(time.Microsecond))
}

// compare sorts out a Library's Tracks and files: a changed file is not
// the one its Track recorded, a moved one went to another path, a gone
// Track has no file, an added file has no Track. It matches them by path.
// A Track without its file that has a new file of the same inode and size
// was moved: a rename keeps both. Hardlinks in other Libraries share the
// inode, so only files of one Library are matched.
func compare(tracks []Track, files []LibraryFile) differences {
	byPath := make(map[string]*Track, len(tracks))
	for n := range tracks {
		byPath[tracks[n].Path] = &tracks[n]
	}
	var d differences
	found := make(map[string]bool, len(files))
	type identity struct {
		inode uint64
		size  int64
	}
	unknown := map[identity][]LibraryFile{}
	for _, file := range files {
		if _, ok := file.Format(); !ok {
			continue
		}
		found[file.Path] = true
		track, ok := byPath[file.Path]
		switch {
		case !ok:
			id := identity{file.Inode, file.Size}
			unknown[id] = append(unknown[id], file)
			d.added = append(d.added, file)
		case !track.Holds(file):
			d.changed = append(d.changed, fileOf{track, file})
		}
	}

	moved := map[string]bool{}
	for n := range tracks {
		track := &tracks[n]
		if found[track.Path] {
			continue
		}
		id := identity{track.FileInode, track.Size}
		if candidates := unknown[id]; track.recorded() && len(candidates) > 0 {
			unknown[id] = candidates[1:]
			moved[candidates[0].Path] = true
			d.moved = append(d.moved, fileOf{track, candidates[0]})
			continue
		}
		d.gone = append(d.gone, track)
	}
	d.added = slices.DeleteFunc(d.added, func(file LibraryFile) bool { return moved[file.Path] })
	return d
}
