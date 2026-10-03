package installer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?$`)

type version struct {
	core       [3]int
	prerelease string
	text       string
}

func parseVersion(text string) (version, error) {
	m := versionPattern.FindStringSubmatch(text)
	if m == nil {
		return version{}, fmt.Errorf("%q is not a release version", text)
	}
	v := version{prerelease: m[4], text: text}
	for i := range v.core {
		v.core[i], _ = strconv.Atoi(m[i+1])
	}
	return v, nil
}

func (v version) String() string { return v.text }

// compare orders versions the way semantic versioning does.
func (v version) compare(o version) int {
	for i := range v.core {
		if v.core[i] != o.core[i] {
			return v.core[i] - o.core[i]
		}
	}
	switch {
	case v.prerelease == o.prerelease:
		return 0
	case v.prerelease == "":
		return 1
	case o.prerelease == "":
		return -1
	}
	a, b := strings.Split(v.prerelease, "."), strings.Split(o.prerelease, ".")
	for i := 0; i < len(a) && i < len(b); i++ {
		x, xErr := strconv.Atoi(a[i])
		y, yErr := strconv.Atoi(b[i])
		switch {
		case xErr == nil && yErr == nil && x != y:
			return x - y
		case xErr == nil && yErr != nil:
			return -1
		case xErr != nil && yErr == nil:
			return 1
		case a[i] != b[i]:
			return strings.Compare(a[i], b[i])
		}
	}
	return len(a) - len(b)
}
