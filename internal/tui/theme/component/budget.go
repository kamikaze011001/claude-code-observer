package component

import "fmt"

// Builder accumulates column widths and reports overflow before rendering.
type Builder struct {
	total int
	cols  map[string]int
	used  int
}

// Budget returns a width budgeter targeting `total` cells.
func Budget(total int) *Builder {
	return &Builder{total: total, cols: map[string]int{}}
}

// Fixed reserves `w` cells for the named column.
func (b *Builder) Fixed(name string, w int) {
	b.cols[name] = w
	b.used += w
}

// Gutters reserves count×width cells for inter-column spacing.
func (b *Builder) Gutters(count, width int) {
	b.used += count * width
}

// Flex claims the remaining budget for the named column and returns the
// width allocated. Use this for the single column that should expand to
// fill the row.
func (b *Builder) Flex(name string) int {
	w := b.total - b.used
	if w < 0 {
		w = 0
	}
	b.cols[name] = w
	b.used += w
	return w
}

// Remaining returns cells still available.
func (b *Builder) Remaining() int { return b.total - b.used }

// Width looks up the cells allocated to a column.
func (b *Builder) Width(name string) int { return b.cols[name] }

// Validate returns an error iff the sum of allocations exceeds total.
func (b *Builder) Validate() error {
	if b.used > b.total {
		return fmt.Errorf("budget overflow: allocated %d > total %d", b.used, b.total)
	}
	return nil
}
