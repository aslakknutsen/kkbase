package testing

// ExpectedNode represents an expected node creation in tests
type ExpectedNode struct {
	Type       string
	ID         string
	Properties map[string]interface{} // nil = don't check properties
}

// ExpectedEdge represents an expected edge creation in tests
type ExpectedEdge struct {
	FromType   string
	FromID     string
	EdgeType   string
	ToType     string
	ToID       string
	Properties map[string]interface{} // nil = don't check properties
}
