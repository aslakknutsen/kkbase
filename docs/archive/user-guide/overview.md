# kkbase Overview

## What It Does

kkbase is a Kubernetes operator that watches cluster resources and syncs them to a Neo4j graph database, creating a real-time knowledge graph of your entire cluster topology.

It transforms your Kubernetes cluster into a queryable, interconnected model where every resource (Pod, Service, Deployment) and relationship (manages, selects, routes-to) is explicitly represented. This enables powerful diagnostic capabilities, dependency analysis, and serves as the foundation for autonomous troubleshooting agents.

## Key Features

- **Real-time Synchronization** - Continuously watches Kubernetes API and updates the graph as resources change
- **Complete Topology** - Models all core Kubernetes resources plus extensions (Gateway API, Istio)
- **Relationship Mapping** - Automatically discovers and creates edges between related resources
- **Extensible** - Plugin architecture for custom resources and CRDs
- **Query-Driven** - Use Cypher to ask complex questions about your cluster
- **Agent-Ready** - Designed as the knowledge base for autonomous diagnostic systems

## High-Level Architecture

kkbase consists of four main components:

```
┌─────────────────┐
│   Kubernetes    │
│   API Server    │
└────────┬────────┘
         │ watch
         ↓
┌─────────────────┐
│    Watchers     │  ← SharedInformers with local cache
│   (Informers)   │
└────────┬────────┘
         │ events
         ↓
┌─────────────────┐
│    Handlers     │  ← Convert resources to graph nodes/edges
│  (per resource) │
└────────┬────────┘
         │ operations
         ↓
┌─────────────────┐
│   Graph Store   │  ← Abstract interface
│     (Neo4j)     │
└─────────────────┘
```

### 1. Watchers (Informers)

Uses Kubernetes `client-go` SharedInformers to efficiently watch resources:
- Single watch per resource type (not per handler)
- Local caching reduces API server load
- Automatic retry and resync capabilities

### 2. Handlers

Resource-specific logic for graph operations:
- **Core Handlers**: Node, Pod, Service, Deployment, etc. (always enabled)
- **Extension Handlers**: Gateway API, Istio, custom CRDs (optional)
- Each handler converts K8s objects to graph nodes and extracts relationships

### 3. Graph Store

Abstract interface with Neo4j implementation:
- Upsert operations for nodes and edges
- Delete operations for cleanup
- Connection pooling and retry logic
- Health checks

### 4. Neo4j Database

Graph database storing the knowledge graph:
- Nodes represent resources
- Edges represent relationships
- Properties store resource state
- Indexed for fast lookups

## When to Use kkbase

kkbase is ideal for:

- **Autonomous Diagnostic Agents** - Provide agents with a structured world model for reasoning
- **Root Cause Analysis** - Trace failures through dependency chains
- **Impact Analysis** - Understand blast radius of changes or outages
- **Dependency Mapping** - Visualize service dependencies and data flow
- **Compliance Auditing** - Query security policies and configurations
- **Capacity Planning** - Analyze resource utilization patterns
- **Cluster Visualization** - Build interactive topology views

## Resource Identification

All resources use a consistent ID format:

- **Namespaced resources**: `Type/namespace/name`
  - Example: `Pod/default/nginx-abc123`
  - Example: `Service/kube-system/metrics-server`

- **Cluster-scoped resources**: `Type/name`
  - Example: `Node/worker-1`
  - Example: `GatewayClass/istio`

This ensures globally unique identifiers and clearly represents resource hierarchy.

## Data Flow Example

When a new Pod is created:

1. **Kubernetes API** emits a Pod creation event
2. **Watcher** (PodInformer) receives the event
3. **PodHandler** is triggered with the Pod object
4. **Handler** converts Pod to a graph node with properties (status, IP, labels)
5. **Handler** creates edges: Pod → Node (SCHEDULED_ON), Pod → Namespace (IN_NAMESPACE)
6. **Handler** finds related resources (Service) and creates edge: Service → Pod (SELECTS_PODS)
7. **Graph Store** upserts node and edges to Neo4j
8. **Neo4j** updates the graph, making it queryable immediately

## Next Steps

- **[Quick Start](quickstart.md)** - Deploy kkbase in 5 minutes
- **[Installation Guide](installation.md)** - Detailed setup instructions
- **[Query Examples](querying.md)** - Learn how to query the graph
- **[Extensions](extensions.md)** - Enable Gateway API and Istio support
- **[Knowledge Graph Concepts](../concepts/knowledge-graph.md)** - Understand the graph model

