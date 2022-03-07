package workshare_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/internal/workshare"
)

type treeNode struct {
	value    int
	children []*treeNode
}

func buildTree(level int) *treeNode {
	n := &treeNode{
		value: 1,
	}

	if level <= 0 {
		return n
	}

	for i := 0; i < level; i++ {
		n.children = append(n.children, buildTree(level-1))
	}

	return n
}

type computeTreeSumRequest struct {
	input *treeNode

	result int
	err    error
}

func dispatchComputeTreeSumRequest(w *workshare.Pool, input interface{}) {
	req := input.(*computeTreeSumRequest)

	if w.ActiveWorkers() == 0 {
		panic("unexpected worker count")
	}

	res, err := computeTreeSum(w, req.input)
	if err != nil {
		req.err = err
		return
	}

	req.result = res
}

func computeTreeSum(workPool *workshare.Pool, n *treeNode) (int, error) {
	total := n.value

	var cs workshare.AsyncGroup

	for _, child := range n.children {
		if cs.CanShareWork(workPool) {
			// run the request on another goroutine, the results will be available
			cs.RunAsync(workPool, dispatchComputeTreeSumRequest, &computeTreeSumRequest{
				input: child,
			})
		} else {
			chtot, err := computeTreeSum(workPool, child)
			if err != nil {
				return 0, err
			}

			total += chtot
		}
	}

	for _, req := range cs.Wait() {
		twr := req.(*computeTreeSumRequest)

		if twr.err != nil {
			return 0, twr.err
		}

		total += twr.result
	}

	return total, nil
}

func TestComputeTreeSum10(t *testing.T) {
	testComputeTreeSum(t, 10)
}

func TestComputeTreeSum1(t *testing.T) {
	testComputeTreeSum(t, 1)
}

func TestComputeTreeSum0(t *testing.T) {
	testComputeTreeSum(t, 0)
}

func TestComputeTreeSumNegative(t *testing.T) {
	testComputeTreeSum(t, -1)
}

// nolint:thelper
func testComputeTreeSum(t *testing.T, numWorkers int) {
	w := workshare.NewPool(numWorkers)
	defer w.Close()

	n := buildTree(6)

	sum, err := computeTreeSum(w, n)
	require.NoError(t, err)
	require.Equal(t, 1957, sum)
}

var treeToWalk = buildTree(6)

func BenchmarkComputeTreeSum(b *testing.B) {
	w := workshare.NewPool(10)
	defer w.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		computeTreeSum(w, treeToWalk)
	}
}