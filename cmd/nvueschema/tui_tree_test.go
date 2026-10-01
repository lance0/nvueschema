package main

import (
	"fmt"
	"testing"
	"time"

	"nemith.io/nvueschema"
)

func deepTree(depth int) *nvueschema.Node {
	root := &nvueschema.Node{Name: "leaf", TypeSegs: []nvueschema.TypeSegment{{Text: "string"}}}
	for i := 0; i < depth; i++ {
		root = &nvueschema.Node{Name: fmt.Sprintf("node%d", i), TypeSegs: []nvueschema.TypeSegment{{Text: "object"}}, Children: []*nvueschema.Node{root}}
	}
	return root
}

func TestExpandAllDeepTree(t *testing.T) {
	const depth = 64
	tree := newTreeState(deepTree(depth))
	done := make(chan struct{})
	go func() { tree.expandAll(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expanding 65 nodes did not finish; check for repeated recursive visits")
	}
	if len(tree.rows) != depth {
		t.Fatalf("rows=%d, want %d", len(tree.rows), depth)
	}
	for _, row := range tree.rows {
		if len(row.effective.Children) > 0 && !row.expanded {
			t.Errorf("%s not expanded", row.path)
		}
	}
	tree.collapseAll()
	if len(tree.rows) != 1 {
		t.Fatalf("collapse leaves %d rows", len(tree.rows))
	}
	tree.expandAll()
	if len(tree.rows) != depth {
		t.Fatalf("second expansion leaves %d rows", len(tree.rows))
	}
}

func TestExpandAllCollapsedChain(t *testing.T) {
	branch := &nvueschema.Node{Name: "branch", Children: []*nvueschema.Node{{Name: "left"}, {Name: "right"}}}
	root := &nvueschema.Node{Name: "root", Children: []*nvueschema.Node{{Name: "wrapper", Children: []*nvueschema.Node{branch}}}}
	tree := newTreeState(root)
	tree.expandAll()
	if len(tree.rows) != 3 || tree.rows[0].collapsedName != "wrapper.branch" || !tree.rows[0].expanded {
		t.Fatalf("unexpected collapsed chain: %+v", tree.rows)
	}
}

func BenchmarkTreeExpandAll(b *testing.B) {
	for _, depth := range []int{16, 64, 256} {
		b.Run(fmt.Sprint(depth), func(b *testing.B) {
			tree := newTreeState(deepTree(depth))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tree.expandAll()
			}
		})
	}
}
