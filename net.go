package avalanche

type node struct {
	id NodeID
}

func newNode(id NodeID) *node {
	return &node{id: id}
}

type Connman struct {
	nodes map[NodeID]*node
}

func NewConnman() *Connman {
	return &Connman{
		nodes: map[NodeID]*node{},
	}
}

func (c *Connman) AddNode(id NodeID) {
	c.nodes[id] = newNode(id)
}

func (c *Connman) NodesIDs() []NodeID {
	nodeIDs := make([]NodeID, 0, len(c.nodes))
	for nodeID := range c.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}
