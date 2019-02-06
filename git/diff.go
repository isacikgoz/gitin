package git

import (
	"regexp"
	"strings"

	"github.com/fatih/color"
)

type Diff struct {
	deltas []*DiffDelta
	stats  []string
	patchs []string
}

func (d *Diff) Deltas() []*DiffDelta {
	return d.deltas
}

func (d *Diff) Patches() []string {
	return d.patchs
}

func (d *Diff) Stats() []string {
	return d.stats
}

type DiffDelta struct {
	Status  int
	OldFile *DiffFile
	NewFile *DiffFile
	Patch   string
}

func (d *DiffDelta) String() string {
	var s string
	s = s + deltaType(d.Status) + " " // strconv.Itoa(d.Status) + " "
	if len(d.OldFile.Path) > 0 && len(d.NewFile.Path) > 0 {
		if d.OldFile.Path == d.NewFile.Path {
			s = s + d.OldFile.Path //+ " " + d.OldFile.Hash[:7] + ".." + d.NewFile.Hash[:7]
		} else {
			s = s + d.OldFile.Path + " -> " + d.NewFile.Path
		}
	}
	return s
}

// FileStatArgs returns git command args for getting diff
func (d *DiffDelta) FileStatArgs(c *Commit) []string {
	args := []string{"diff"}
	if c.commit.Parent(0) == nil {
		args = []string{"show", "--oneline", "--patch"}
		args = append(args, c.Hash)
	} else {
		parent := c.commit.Parent(0).AsObject().Id().String()
		args = append(args, parent+".."+c.Hash)
	}
	args = append(args, d.OldFile.Path)

	return args
}

// colorize the plain diff text collected from system output
// the style is near to original diff command
func colorizeDiff(original string) (colorized []string) {

	var (
		green = color.New(color.FgGreen)
		red   = color.New(color.FgRed)
		cyan  = color.New(color.FgCyan)
	)

	colorized = strings.Split(original, "\n")
	re := regexp.MustCompile(`@@ .+ @@`)
	for i, line := range colorized {
		if len(line) > 0 {
			if line[0] == '-' {
				colorized[i] = red.Sprint(line)
			} else if line[0] == '+' {
				colorized[i] = green.Sprint(line)
			} else if re.MatchString(line) {
				s := re.FindString(line)
				colorized[i] = cyan.Sprint(s) + line[len(s):]
			} else {
				continue
			}
		} else {
			continue
		}
	}
	return colorized
}

func (d *DiffDelta) PatchString() string {
	return strings.Join(colorizeDiff(d.Patch), "\n")
}

type DiffFile struct {
	Path string
	Hash string
}

func deltaType(i int) string {
	switch i {
	case 0:
		return "u"
	case 1:
		return "a"

	case 2:
		return "d"

	case 3:
		return "m"

	case 4:
		return "r"

	case 5:
		return "c"

	case 6:
		return "i"

	case 7:
		return "?"

	case 8:
		return "!"

	case 9:
		return "x"

	case 10:
		return "x"
	default:
		return ""
	}

}
