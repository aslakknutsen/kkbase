# Querying the Knowledge Graph

Learn how to query the kkbase knowledge graph using Cypher.

## What's in This Section?

| Guide | Purpose | Level |
|-------|---------|-------|
| [Basics](basics.md) | Introduction to Cypher and common patterns | Beginner |
| [Relationships](relationships.md) | Traversing edges and multi-hop queries | Intermediate |
| [Advanced](advanced.md) | Complex patterns and optimization | Advanced |
| [RCA Patterns](rca-patterns.md) | Root cause analysis query patterns | Advanced |

## Quick Start

### Access Neo4j Browser

```bash
kubectl port-forward svc/neo4j 7474:7474
open http://localhost:7474
```

### Your First Query

```cypher
// See what's in the graph
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC
```

## Query Tools

### Neo4j Browser

- **Visual query builder**
- **Graph visualization**
- **Query history**
- **Built-in documentation**

Access: http://localhost:7474

### MCP Server Tools

Query via AI agents:

```python
# In Cursor or Claude
"Query the kkbase graph to show all pods in the default namespace"
```

### cypher-shell (CLI)

```bash
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme \
  "MATCH (p:Pod) RETURN p.name LIMIT 10"
```

## Common Use Cases

### 1. Resource Discovery

Find resources by type, namespace, labels:
- [Basic Queries](basics.md#resource-queries)

### 2. Dependency Mapping

Trace relationships between services:
- [Relationship Traversal](relationships.md#dependency-chains)

### 3. Impact Analysis

Understand blast radius of failures:
- [Advanced Patterns](advanced.md#impact-analysis)

### 4. Root Cause Analysis

Diagnose issues by following causal chains:
- [RCA Patterns](rca-patterns.md)

## Learning Path

**New to Cypher?**
1. Start with [Basics](basics.md)
2. Practice simple MATCH queries
3. Move to [Relationships](relationships.md)

**Familiar with Cypher?**
1. Review [Advanced](advanced.md) patterns
2. Study [RCA Patterns](rca-patterns.md)
3. Optimize your queries

**Building AI Agents?**
1. Study [RCA Patterns](rca-patterns.md)
2. Learn [Investigation Workflow](../investigations/workflow.md)
3. See [Tools Reference](../../services/mcp-server/tools-reference.md)

## Quick Reference

### Basic Syntax

```cypher
// Find nodes
MATCH (n:Pod)
RETURN n

// Follow relationships
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s, p

// Filter
MATCH (p:Pod)
WHERE p.status = 'Running'
RETURN p

// Count
MATCH (p:Pod)
RETURN count(p)
```

### Common Patterns

See [Basics Guide](basics.md#common-patterns)

## Performance Tips

1. **Use LIMIT** - Restrict result sets
2. **Filter early** - WHERE clauses before traversal
3. **Index on id** - Already indexed automatically
4. **Profile queries** - Use EXPLAIN/PROFILE
5. **Avoid SELECT \*** - Return only needed fields

See [Advanced Guide](advanced.md#performance-optimization)

## See Also

- [Graph Schema](../../reference/graph-schema.md) - Node and edge types
- [System Architecture](../../ARCHITECTURE.md) - How the graph is built
- [Core Concepts](../../getting-started/concepts.md) - Understanding the knowledge graph

