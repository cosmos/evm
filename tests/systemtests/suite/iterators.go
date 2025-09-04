package suite

type NodeIterator struct {
	nodes []string
}

func NewNodeIterator(nodeEntries []string) *NodeIterator {
	return &NodeIterator{
		nodes: nodeEntries,
	}
}

func (iter *NodeIterator) Node() (nodeID string) {
	// return current node
	return iter.nodes[0]
}

func (iter *NodeIterator) Next() *NodeIterator {
	if len(iter.nodes) == 0 {
		panic("node entries are empty")
	}
	iter.nodes = iter.nodes[1:]
	return iter
}

func (iter *NodeIterator) IsEmpty() bool {
	return len(iter.nodes) == 0
}
