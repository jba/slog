// Package withsupport provides support for Handler.WithAttr and
// Handler.WithGroup.
package withsupport

import "golang.org/x/exp/slog"

// GroupOrAttrs holds either a group name or a list of slog.Attrs.
type GroupOrAttrs struct {
	Group string      // group name if non-empty
	Attrs []slog.Attr // attrs if non-empty
	Next  *GroupOrAttrs
}

// WithGroup returns a GroupOrAttrs that includes the given group.
func (g *GroupOrAttrs) WithGroup(name string) *GroupOrAttrs {
	if name == "" {
		return g
	}
	return &GroupOrAttrs{
		Group: name,
		Next:  g,
	}
}

// WithAttrs returns a GroupOrAttrs that includes the given attrs.
func (g *GroupOrAttrs) WithAttrs(attrs []slog.Attr) *GroupOrAttrs {
	if len(attrs) == 0 {
		return g
	}
	return &GroupOrAttrs{
		Attrs: attrs,
		Next:  g,
	}
}

// Apply calls f on each Attr in g. The first argument to f is the list
// of groups that precede the Attr.
// Apply returns the complete list of groups.
func (g *GroupOrAttrs) Apply(f func(groups []string, a slog.Attr)) []string {
	var groups []string

	var rec func(*GroupOrAttrs)
	rec = func(g *GroupOrAttrs) {
		if g == nil {
			return
		}
		rec(g.Next)
		if g.Group != "" {
			groups = append(groups, g.Group)
		} else {
			for _, a := range g.Attrs {
				f(groups, a)
			}
		}
	}
	rec(g)

	return groups
}

// Collect returns a slice of the GroupOrAttrs in reverse order.
func (g *GroupOrAttrs) Collect() []*GroupOrAttrs {
	n := 0
	for ga := g; ga != nil; ga = ga.Next {
		n++
	}
	res := make([]*GroupOrAttrs, n)
	i := 0
	for ga := g; ga != nil; ga = ga.Next {
		res[len(res)-i-1] = ga
		i++
	}
	return res
}
