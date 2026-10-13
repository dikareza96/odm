//go:build windows

package ui

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"

	"odm/internal/engine"
	"odm/internal/osutil"
)

func openPath(p string)   { _ = osutil.Open(p) }
func revealPath(p string) { _ = osutil.Reveal(p) }

func humanBytes(n int64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func rateText(d engine.Info) string {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 {
		return ""
	}
	return humanBytes(d.Speed) + "/s"
}

func etaText(d engine.Info) string {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 || d.Total <= 0 {
		return ""
	}
	secs := float64(d.Total-d.Done) / float64(d.Speed)
	m, s := int(secs)/60, int(secs)%60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

/* ---------- table model ---------- */

type downloadModel struct {
	walk.TableModelBase
	app *application
}

func (m *downloadModel) RowCount() int {
	m.app.mu.Lock()
	defer m.app.mu.Unlock()
	return len(m.app.items)
}

func (m *downloadModel) Value(row, col int) interface{} {
	m.app.mu.Lock()
	defer m.app.mu.Unlock()
	if row < 0 || row >= len(m.app.items) {
		return ""
	}
	d := m.app.items[row]
	switch col {
	case 0:
		return d.FileName
	case 1:
		return humanBytes(d.Total)
	case 2:
		if d.Total > 0 {
			return fmt.Sprintf("%.1f%%", d.Progress)
		}
		return ""
	case 3:
		return string(d.Status)
	case 4:
		return etaText(d)
	case 5:
		return rateText(d)
	case 6:
		return d.Dir
	}
	return ""
}

/* ---------- category tree ---------- */

type catItem struct {
	text     string
	kind     string
	value    string
	parent   *catItem
	children []*catItem
}

func (i *catItem) Text() string { return i.text }

func (i *catItem) Parent() walk.TreeItem {
	if i.parent == nil {
		return nil
	}
	return i.parent
}

func (i *catItem) ChildCount() int { return len(i.children) }

func (i *catItem) ChildAt(index int) walk.TreeItem { return i.children[index] }

type catModel struct {
	walk.TreeModelBase
	roots []*catItem
	sig   string
}

func (m *catModel) RootCount() int { return len(m.roots) }

func (m *catModel) RootAt(index int) walk.TreeItem { return m.roots[index] }

func buildCategoryTree(items []engine.Info) *catModel {
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range items {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
	}

	all := &catItem{text: fmt.Sprintf("All Downloads (%d)", len(items)), kind: "all"}
	cats := &catItem{text: "Categories"}
	for _, name := range engine.Categories() {
		cats.children = append(cats.children, &catItem{
			text: fmt.Sprintf("%s (%d)", name, counts[name]), kind: "category", value: name, parent: cats,
		})
	}
	status := &catItem{text: "Status"}
	status.children = append(status.children,
		&catItem{text: fmt.Sprintf("Unfinished (%d)", unfinished), kind: "unfinished", parent: status},
		&catItem{text: fmt.Sprintf("Finished (%d)", finished), kind: "finished", parent: status},
	)

	m := &catModel{roots: []*catItem{all, cats, status}}
	var sb strings.Builder
	for _, r := range m.roots {
		sb.WriteString(r.text)
		for _, c := range r.children {
			sb.WriteString(c.text)
		}
	}
	m.sig = sb.String()
	return m
}
