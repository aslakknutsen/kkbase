# Web Dashboard

The embedded web dashboard provides real-time monitoring and visualization of AI agent investigation sessions.

## Overview

The dashboard is a React/TypeScript application served by the MCP server that displays:

- **Active sessions** with status and progress
- **Hypothesis evolution** showing diagnostic reasoning
- **Blast zone visualization** of affected resources
- **Findings** with severity and evidence
- **Query history** with reasoning
- **Recommendations** with action items
- **Real-time updates** via Server-Sent Events (SSE)

## Access

### Local Development

```bash
# Port forward to MCP server
kubectl port-forward svc/kkbase-mcp-server 8080:8080

# Open browser
open http://localhost:8080/
```

### Production (via Ingress)

```
https://kkbase.example.com/
```

## Dashboard Layout

```
┌─────────────────────────────────────────────────────┐
│  kkbase Dashboard                    [Refresh Icon] │
├───────────────┬─────────────────────────────────────┤
│               │                                     │
│  Sidebar      │    Main Content Area                │
│               │                                     │
│ Active        │  Selected Session Details:          │
│ Investigations│  ┌──────────────────────────────┐  │
│               │  │ Session Header               │  │
│ ○ Session 1   │  │ - ID, Status, Duration       │  │
│ ○ Session 2   │  └──────────────────────────────┘  │
│ ○ Session 3   │  ┌──────────────────────────────┐  │
│               │  │ Current Hypothesis           │  │
│ [No Active]   │  │ - Stage, Theory Text         │  │
│               │  └──────────────────────────────┘  │
│               │  ┌──────────────────────────────┐  │
│               │  │ Blast Zone Graph             │  │
│               │  │ - Nodes (resources)          │  │
│               │  │ - Edges (relationships)      │  │
│               │  │ - Status indicators          │  │
│               │  └──────────────────────────────┘  │
│               │  ┌──────────────────────────────┐  │
│               │  │ Findings                     │  │
│               │  │ - Type, Severity             │  │
│               │  │ - Description, Evidence      │  │
│               │  └──────────────────────────────┘  │
│               │  ┌──────────────────────────────┐  │
│               │  │ Recommendations              │  │
│               │  │ - Priority, Title            │  │
│               │  │ - Action Items               │  │
│               │  └──────────────────────────────┘  │
│               │  ┌──────────────────────────────┐  │
│               │  │ Query History                │  │
│               │  │ - Reasoning, Results         │  │
│               │  └──────────────────────────────┘  │
│               │  ┌──────────────────────────────┐  │
│               │  │ Timeline                     │  │
│               │  │ - Chronological Events       │  │
│               │  └──────────────────────────────┘  │
└───────────────┴─────────────────────────────────────┘
```

## Features

### 1. Active Sessions List (Sidebar)

Shows all currently active investigation sessions:

- **Session ID** (truncated for display)
- **Initial symptom** description
- **Status** indicator (active, completed, timeout)
- **Start time** (relative, e.g., "5 minutes ago")
- **Click to view** full session details

**Real-time**: New sessions appear automatically via SSE.

### 2. Session Header

When a session is selected:

- **Full session ID**
- **Status badge** (Active, Completed, Timeout, Incomplete)
- **Start time** (absolute timestamp)
- **Duration** (if completed)
- **Summary** (if provided at completion)

### 3. Current Hypothesis

Displays the agent's current diagnostic theory:

- **Stage number** (1, 2, 3, etc.)
- **Hypothesis text** explaining the current theory
- **Previous hypotheses** (collapsed, expandable)
- **Status** (active, superseded)

**Updates**: New hypothesis supersedes previous, triggers blast zone recalculation.

### 4. Blast Zone Visualization

Interactive graph showing affected resources:

**Node Colors**:
- **Red** - Failed/critical resources
- **Yellow** - Degraded/warning resources
- **Green** - Healthy but affected resources
- **Gray** - Related/contextual resources

**Edge Styles**:
- **Solid** - Direct relationships
- **Dashed** - Indirect relationships
- **Animated** - Failed connections

**Interactions**:
- **Hover** - Show node details
- **Click** - Highlight connections
- **Zoom** - Mouse wheel or pinch
- **Pan** - Click and drag

**Metrics**:
- Total affected resources count
- Hop radius (how far impact spreads)
- Resource breakdown by type

### 5. Findings List

All issues discovered during investigation:

**Finding Card**:
- **Type** (unhealthy_pod, failed_dependency, error_spike, etc.)
- **Severity** badge (Critical, Warning, Info)
- **Resource ID** affected
- **Description** of the issue
- **Evidence** (expandable JSON)
- **Detection method** (automatic, agent_recorded)
- **Timestamp**

**Filtering** (planned):
- By severity
- By type
- By resource

**Sorting** (planned):
- By severity
- By timestamp

### 6. Recommendations Panel

Actionable next steps recorded by the agent:

**Recommendation Card**:
- **Type** badge (Root Cause Fix, Preventive Action, Optimization, Monitoring, Cleanup)
- **Priority** indicator (Critical, High, Medium, Low)
- **Title** - Short summary
- **Description** - Detailed explanation
- **Rationale** - Why this recommendation
- **Action Items** - Numbered steps
- **Automation Hint** - Commands/scripts (code formatted)
- **Related Findings** - Links to supporting evidence
- **Estimated Effort** - Time to complete
- **Tags** - Categorization

**Actions** (planned):
- Copy automation hint
- Export recommendation
- Mark as completed

### 7. Query History

All Cypher queries executed in the session:

**Query Entry**:
- **Query number** (#1, #2, #3...)
- **Reasoning** - Why this query was executed
- **Query text** - Cypher code (syntax highlighted)
- **Results** - Count and sample
- **Duration** - Execution time
- **Findings extracted** - Count and types
- **Timestamp**

**Interactions**:
- **Expand/collapse** query text
- **View full results** (in modal)
- **Copy query** to clipboard

### 8. Timeline

Chronological event log:

**Event Types**:
- Session created
- Hypothesis added/updated
- Query executed
- Finding discovered
- Investigation spawned
- Recommendation recorded
- Session completed

**Event Display**:
- **Icon** - Type-specific
- **Timestamp** - Absolute and relative
- **Description** - Event details
- **Related data** - Links to findings, queries, etc.

**Filtering** (planned):
- By event type
- By time range

### 9. Linked Investigations

Metrics investigations spawned during the session:

**Investigation Card**:
- **Investigation ID**
- **Resource** (type and ID)
- **Symptom** being investigated
- **Status** (active, completed)
- **Metrics collected** - Count
- **Timestamp**

**Note**: Investigations are sub-flows within the agent session, not separate investigations.

## Real-Time Updates

The dashboard uses Server-Sent Events (SSE) for live updates:

### Update Flow

```
Agent → MCP Server → Neo4j
                 ↓
           SSE Notification
                 ↓
           Dashboard Auto-Update
```

### Update Frequency

- **SSE connection**: Persistent, bi-directional
- **Poll fallback**: Every 5 seconds if SSE fails
- **Debouncing**: Rapid updates batched (200ms)

### What Triggers Updates

- New session created
- Hypothesis updated
- Query executed
- Finding discovered
- Recommendation recorded
- Session completed

### Connection Status

Dashboard shows connection indicator:
- **Green dot** - SSE connected
- **Yellow dot** - Polling mode
- **Red dot** - Disconnected

## Empty States

### No Active Sessions

```
┌──────────────────────────────────────┐
│                                      │
│     No Active Investigations         │
│                                      │
│  Agent sessions will appear here     │
│  when AI tools create them.          │
│                                      │
│  To test: Ask your AI agent to start│
│  an investigation using kkbase.      │
│                                      │
└──────────────────────────────────────┘
```

### No Findings Yet

```
No findings discovered yet. The agent is still investigating...
```

### No Recommendations

```
No recommendations recorded. Investigation in progress...
```

## Performance

### Optimizations

- **Virtual scrolling** for large lists
- **Lazy loading** of query results
- **Memoization** of expensive computations
- **Debounced updates** for rapid changes
- **Code splitting** for faster initial load

### Resource Usage

- **Memory**: ~50MB in browser
- **Network**: <1KB/sec average (SSE stream)
- **CPU**: Minimal (event-driven updates)

## Browser Support

- **Chrome/Edge**: 90+
- **Firefox**: 88+
- **Safari**: 14+
- **Mobile**: iOS Safari 14+, Chrome Android 90+

## Customization (Future)

Planned customization options:

- **Theme**: Light/dark mode
- **Layout**: Customize panels
- **Filters**: Save filter preferences
- **Notifications**: Browser notifications for events
- **Export**: Download session as JSON/PDF

## Development

### Running Locally

```bash
# Frontend development server
cd frontend
npm install
npm run dev

# Opens http://localhost:5173
# Proxies /mcp to backend at :8080
```

### Building

```bash
cd frontend
npm run build

# Output: frontend/dist/
# Copied to: cmd/mcp-server/frontend/dist/
```

### Architecture

```
frontend/
├── src/
│   ├── components/
│   │   ├── SessionList.tsx
│   │   ├── SessionDetail.tsx
│   │   ├── BlastZone.tsx
│   │   ├── FindingsList.tsx
│   │   ├── RecommendationsList.tsx
│   │   ├── QueryHistory.tsx
│   │   └── Timeline.tsx
│   ├── services/
│   │   └── mcpObserver.ts  # SSE connection
│   ├── types/
│   │   ├── session.ts
│   │   ├── finding.ts
│   │   └── recommendation.ts
│   ├── App.tsx
│   └── main.tsx
├── package.json
└── vite.config.ts
```

## Troubleshooting

### Dashboard Won't Load

```bash
# Check MCP server logs
kubectl logs deployment/kkbase-mcp-server | grep frontend

# Expected: "embedded frontend enabled"

# If missing, frontend not embedded in binary
# Rebuild with frontend: make build-mcp-server
```

### No Sessions Showing

Sessions only appear when created by agents. Test manually:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "start_agent_session",
      "arguments": {
        "symptom": "Test dashboard display"
      }
    }
  }'

# Session should appear in dashboard within 5 seconds
```

### SSE Connection Failed

```bash
# Check browser console for errors
# Try polling fallback (automatic after 3 failed SSE attempts)

# Check CORS if accessing from different origin
# MCP server CORS headers should allow dashboard origin
```

### Blast Zone Not Rendering

```bash
# Check browser console for errors
# Verify findings have AFFECTS relationships

# Test in Neo4j:
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme \
  "MATCH (f:Finding)-[:AFFECTS]->(r) RETURN count(r)"

# Should return count > 0
```

## Best Practices

1. **Keep dashboard open** during investigations to monitor progress
2. **Use multiple views** - Open multiple sessions in tabs
3. **Export data** (when available) for incident reports
4. **Monitor connection** - Check SSE indicator
5. **Review timeline** - Understand investigation flow
6. **Check recommendations** - Actionable next steps

## See Also

- **[MCP Server README](README.md)** - Service overview
- **[Tools Reference](tools-reference.md)** - Complete API
- **[Investigation Workflow](../../guides/investigations/workflow.md)** - How agents use the dashboard
- **[Local Development Guide](../../getting-started/local-development.md)** - Frontend development

