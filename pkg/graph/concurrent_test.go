package graph

import (
	"fmt"
	"sync"
	"testing"

	"github.com/karunapuram/pathcollapse/pkg/model"
)

// TestConcurrentAddNode verifies that concurrent AddNode calls are race-free.
func TestConcurrentAddNode(t *testing.T) {
	const goroutines = 50
	const nodesPerGoroutine = 20

	g := New()
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < nodesPerGoroutine; j++ {
				id := fmt.Sprintf("n-%d-%d", i, j)
				if err := g.AddNode(model.NewNode(id, model.NodeUser, id)); err != nil {
					t.Errorf("AddNode(%s): %v", id, err)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := g.NodeCount(); got != goroutines*nodesPerGoroutine {
		t.Fatalf("expected %d nodes, got %d", goroutines*nodesPerGoroutine, got)
	}
}

// TestConcurrentReadWrite verifies that concurrent reads and writes don't race.
func TestConcurrentReadWrite(t *testing.T) {
	g := New()
	const preloaded = 100
	for i := 0; i < preloaded; i++ {
		id := fmt.Sprintf("pre-%d", i)
		g.AddNode(model.NewNode(id, model.NodeUser, id))
	}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				id := fmt.Sprintf("w-%d-%d", i, j)
				_ = g.AddNode(model.NewNode(id, model.NodeUser, id))
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < preloaded; j++ {
				_ = g.GetNode(fmt.Sprintf("pre-%d", j))
				_ = g.NodeCount()
			}
			_ = g.Nodes()
		}()
	}

	wg.Wait()
}

// TestConcurrentAddEdge verifies that concurrent AddEdge calls are race-free.
func TestConcurrentAddEdge(t *testing.T) {
	g := New()
	const n = 100
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("node-%d", i)
		g.AddNode(model.NewNode(id, model.NodeUser, id))
	}

	var wg sync.WaitGroup
	for i := 0; i < n-1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src := fmt.Sprintf("node-%d", i)
			tgt := fmt.Sprintf("node-%d", i+1)
			eid := fmt.Sprintf("edge-%d", i)
			_ = g.AddEdge(model.NewEdge(eid, model.EdgeMemberOf, src, tgt))
		}(i)
	}
	wg.Wait()

	if got := g.EdgeCount(); got != n-1 {
		t.Fatalf("expected %d edges, got %d", n-1, got)
	}
}

// TestConcurrentRemoveAndRead verifies that concurrent RemoveNode and read operations don't race.
func TestConcurrentRemoveAndRead(t *testing.T) {
	g := New()
	const n = 200
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("r-%d", i)
		g.AddNode(model.NewNode(id, model.NodeUser, id))
	}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				g.RemoveNode(fmt.Sprintf("r-%d", i*10+j))
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				_ = g.GetNode(fmt.Sprintf("r-%d", j%n))
			}
			_ = g.Nodes()
			_ = g.NodeCount()
		}()
	}

	wg.Wait()
}

// TestConcurrentNeighbors verifies concurrent Neighbors/ReverseNeighbors calls don't race.
func TestConcurrentNeighbors(t *testing.T) {
	g := New()
	g.AddNode(model.NewNode("hub", model.NodeGroup, "hub"))
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("spoke-%d", i)
		g.AddNode(model.NewNode(id, model.NodeUser, id))
		g.AddEdge(model.NewEdge(fmt.Sprintf("e-%d", i), model.EdgeMemberOf, "hub", id))
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = g.Neighbors("hub")
				_ = g.ReverseNeighbors(fmt.Sprintf("spoke-%d", j%50))
			}
		}()
	}
	wg.Wait()
}
