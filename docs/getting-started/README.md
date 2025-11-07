# Getting Started with kkbase

Welcome to kkbase! This section will help you get up and running quickly based on your needs.

## Quick Navigation

### New to kkbase?

Start here to understand what kkbase is and how it works:

**[System Overview](overview.md)** - Learn about the three services and when to use them

**[Core Concepts](concepts.md)** - Understand the knowledge graph model

### Ready to Deploy?

Choose the deployment path that matches your needs:

#### Option 1: Knowledge Graph Only

**Goal**: Query your cluster as a graph database

**You Need**: Watcher + Neo4j

**Best For**:
- Learning the knowledge graph model
- Custom integrations
- Manual cluster analysis

**Start Here**: [Minimal Quick Start](quickstart-minimal.md)

#### Option 2: AI-Assisted Investigation

**Goal**: Use AI agents (Cursor, Claude) to investigate cluster issues

**You Need**: Watcher + MCP Server + Neo4j + AI tool integration

**Best For**:
- Development teams
- On-demand troubleshooting
- Learning autonomous diagnostics

**Start Here**: [Full Stack Quick Start](quickstart-with-agent.md)

### Developing with kkbase?

**Goal**: Contribute to kkbase or extend it

**You Need**: Local development environment

**Best For**:
- Adding custom handlers
- Developing new MCP tools
- Frontend development

**Start Here**: [Local Development Guide](local-development.md)

## What's in This Section?

| Document | Purpose | Time to Read |
|----------|---------|--------------|
| [Overview](overview.md) | Understand the system | 10 min |
| [Concepts](concepts.md) | Learn the knowledge graph model | 15 min |
| [Minimal Quick Start](quickstart-minimal.md) | Deploy watcher + Neo4j | 10 min |
| [Full Stack Quick Start](quickstart-with-agent.md) | Deploy complete system | 20 min |
| [Local Development](local-development.md) | Set up dev environment | 15 min |

## Decision Tree

Not sure which path to take? Answer these questions:

### 1. Do you want AI agents to investigate automatically?

**No** → Start with [Minimal Quick Start](quickstart-minimal.md)
- You'll have a queryable knowledge graph
- Use Neo4j browser or custom queries
- Can upgrade to full stack later

**Yes** → Continue to question 2

### 2. Do you want fully autonomous investigation (webhooks)?

**No** → Use [Full Stack Quick Start](quickstart-with-agent.md)
- AI agents work on-demand (Cursor, Claude)
- You trigger investigations manually
- Great for development teams

**Yes** → Use [Full Stack Quick Start](quickstart-with-agent.md) + [Agent Configuration](../services/agent/configuration.md)
- Set up webhook integration
- Agents respond to alerts automatically
- Production-ready autonomous diagnostics

### 3. Are you developing kkbase features?

**Yes** → Use [Local Development Guide](local-development.md)
- Hot-reload development
- Test changes quickly
- Contribute features

## Common Questions

### Can I start minimal and upgrade later?

Yes! Start with just the watcher to build the knowledge graph. Add MCP server and agent service later without losing data.

### What are the resource requirements?

**Minimal (Watcher + Neo4j)**:
- 512MB RAM
- 1 CPU core
- 2GB disk

**Full Stack (+ MCP Server + Agent)**:
- 1GB RAM
- 1-2 CPU cores
- 2GB disk

### Do I need Prometheus or Jaeger?

No, they're optional:
- **Prometheus**: Enables metrics-based RCA investigations
- **Jaeger**: Adds distributed tracing correlation

Start without them and add later if needed.

### Can I use this in production?

Yes, but consider:
- Add authentication to MCP server
- Use TLS for all connections
- Set up monitoring and alerting
- See [Operations Guide](../guides/operations/) for best practices

## Next Steps After Setup

Once you've completed a quick start guide:

### If you deployed watcher-only:
1. [Query the Graph](../guides/querying/basics.md) - Learn Cypher queries
2. [Explore Extensions](../services/watcher/extensions.md) - Add Gateway API or Istio support
3. [Custom Handlers](../services/watcher/custom-handlers.md) - Track custom CRDs

### If you deployed full stack:
1. [Investigation Workflow](../guides/investigations/workflow.md) - Learn how agents investigate
2. [MCP Tools Reference](../services/mcp-server/tools-reference.md) - Available tools
3. [Best Practices](../guides/investigations/best-practices.md) - Investigation patterns

## Need Help?

- **Troubleshooting**: See [Operations Guide](../guides/operations/troubleshooting.md)
- **Configuration**: See [Configuration Reference](../reference/configuration.md)
- **Architecture**: See [System Architecture](../ARCHITECTURE.md)

## Quick Links

- [System Architecture](../ARCHITECTURE.md)
- [Watcher Service](../services/watcher/)
- [MCP Server](../services/mcp-server/)
- [Agent Service](../services/agent/)

