package mcp

import (
	"strings"
	"testing"
)

func TestValidateReadOnlyQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantError bool
		errorText string
	}{
		{
			name:      "valid MATCH query",
			query:     "MATCH (n:Pod) RETURN n LIMIT 10",
			wantError: false,
		},
		{
			name:      "valid MATCH with WHERE",
			query:     "MATCH (n:Service) WHERE n.namespace = 'default' RETURN n",
			wantError: false,
		},
		{
			name:      "valid complex query",
			query:     "MATCH (p:Pod)-[:SCHEDULED_ON]->(n:Node) RETURN p, n",
			wantError: false,
		},
		{
			name:      "valid query with parameters",
			query:     "MATCH (n:Pod) WHERE n.namespace = $namespace RETURN n",
			wantError: false,
		},
		{
			name:      "empty query",
			query:     "",
			wantError: true,
			errorText: "query cannot be empty",
		},
		{
			name:      "whitespace only query",
			query:     "   \n\t  ",
			wantError: true,
			errorText: "query cannot be empty",
		},
		{
			name:      "CREATE operation",
			query:     "CREATE (n:Pod {name: 'test'}) RETURN n",
			wantError: true,
			errorText: "CREATE",
		},
		{
			name:      "DELETE operation",
			query:     "MATCH (n:Pod) DELETE n",
			wantError: true,
			errorText: "DELETE",
		},
		{
			name:      "SET operation",
			query:     "MATCH (n:Pod) SET n.status = 'Running' RETURN n",
			wantError: true,
			errorText: "SET",
		},
		{
			name:      "MERGE operation",
			query:     "MERGE (n:Pod {name: 'test'}) RETURN n",
			wantError: true,
			errorText: "MERGE",
		},
		{
			name:      "REMOVE operation",
			query:     "MATCH (n:Pod) REMOVE n.label RETURN n",
			wantError: true,
			errorText: "REMOVE",
		},
		{
			name:      "DROP operation",
			query:     "DROP INDEX pod_index",
			wantError: true,
			errorText: "DROP",
		},
		{
			name:      "DETACH DELETE operation",
			query:     "MATCH (n:Pod) DETACH DELETE n",
			wantError: true,
			errorText: "DELETE",
		},
		{
			name:      "lowercase create",
			query:     "match (n:Pod) create (m:Service) return m",
			wantError: true,
			errorText: "CREATE",
		},
		{
			name:      "mixed case DELETE",
			query:     "match (n:Pod) DeLeTe n",
			wantError: true,
			errorText: "DELETE",
		},
		{
			name:      "create in comment (should still be caught)",
			query:     "MATCH (n:Pod) /* CREATE something */ RETURN n",
			wantError: true,
			errorText: "CREATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReadOnlyQuery(tt.query)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateReadOnlyQuery() expected error but got none")
					return
				}
				if tt.errorText != "" && !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("ValidateReadOnlyQuery() error = %v, want error containing %q", err, tt.errorText)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateReadOnlyQuery() unexpected error = %v", err)
				}
			}
		})
	}
}
