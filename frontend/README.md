# KKBase Agent Investigation Dashboard

A React-based web dashboard for observing AI agent investigation sessions in real-time.

## Overview

This dashboard provides a read-only interface to observe agent diagnostic sessions. When an AI agent (running in Cursor) investigates a Kubernetes cluster issue, this dashboard visualizes:

- **Active Sessions**: List of ongoing investigations
- **Current Hypothesis**: The agent's current theory about the problem
- **Blast Zone Graph**: Visual representation of affected resources
- **Findings**: Discovered issues (failed services, unhealthy pods, etc.)
- **Query History**: All Cypher queries executed by the agent
- **Timeline**: Chronological view of investigation events

## Architecture

### Technology Stack

- **React 18** - UI framework
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **React Flow** - Graph visualization
- **Tailwind CSS** - Styling
- **Dagre** - Graph layout algorithm

### Communication

The dashboard communicates with the backend MCP server using:

1. **HTTP POST requests** to MCP tools (read-only endpoints)
2. **Polling** for updates (5-second interval by default)

The dashboard is purely read-only and does not initiate investigations.

## Development

### Prerequisites

- Node.js 18+ and npm
- Running MCP server backend

### Setup

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

The dev server will run on http://localhost:3000 and proxy MCP requests to http://localhost:8080.

### Build

```bash
# Production build
npm run build
```

Output will be in `dist/` directory.

### Integration with Go Backend

The frontend is embedded in the Go binary using `go:embed`. The Makefile handles:

1. Building the frontend (`make build-frontend`)
2. Copying dist to `cmd/mcp-server/frontend/dist`
3. Building the Go binary with embedded frontend

## Components

### Core Components

- **`App.tsx`** - Main application shell
- **`SessionList.tsx`** - Sidebar with active sessions
- **`SessionView.tsx`** - Main content area for session details
- **`BlastZoneGraph.tsx`** - React Flow graph visualization
- **`HypothesisPanel.tsx`** - Current hypothesis display
- **`FindingsList.tsx`** - List of discovered findings
- **`QueryList.tsx`** - Query execution history
- **`Timeline.tsx`** - Chronological event timeline
- **`EmptyState.tsx`** - Placeholder when no sessions active

### Services

- **`mcpObserver.ts`** - MCP communication layer
  - Calls read-only MCP tools
  - Implements polling for updates
  - Handles data transformation

### Utilities

- **`graphLayout.ts`** - Dagre-based graph layout algorithm
  - Auto-positions nodes in blast zone graph
  - Applies status-based styling

## MCP Communication

The dashboard calls these MCP tools:

### `get_active_sessions`
Returns list of active investigation sessions.

**Response:**
```json
[
  {
    "id": "session-123",
    "initial_symptom": "Order service failing",
    "created_at": "2025-11-03T14:30:00Z",
    "query_count": 5,
    "finding_count": 3,
    "current_stage": 2
  }
]
```

### `get_session_details`
Returns complete session state.

**Input:** `{ session_id: string }`

**Response:**
```json
{
  "session": { ... },
  "hypotheses": [ ... ],
  "queries": [ ... ],
  "findings": [ ... ],
  "investigations": [ ... ],
  "current_hypothesis": { ... }
}
```

### `get_blast_zone`
Returns graph of affected resources.

**Input:** `{ session_id: string }`

**Response:**
```json
{
  "session_id": "session-123",
  "timestamp": "2025-11-03T14:35:00Z",
  "nodes": [
    {
      "id": "Pod/prod/order-service-abc",
      "label": "order-service-abc",
      "type": "Pod",
      "status": "failed"
    }
  ],
  "edges": [
    {
      "source": "Service/prod/order-service",
      "target": "Pod/prod/order-service-abc",
      "type": "SELECTS_PODS",
      "status": "ok"
    }
  ],
  "affected_count": 5,
  "impact_radius": 3
}
```

### `get_session_timeline`
Returns chronological event list.

**Input:** `{ session_id: string }`

**Response:**
```json
[
  {
    "timestamp": "2025-11-03T14:30:05Z",
    "type": "hypothesis",
    "data": { "text": "Service may be failing due to..." }
  },
  {
    "timestamp": "2025-11-03T14:30:12Z",
    "type": "query",
    "data": { "query": "MATCH...", "reasoning": "..." }
  }
]
```

## Styling

The dashboard uses Tailwind CSS with custom classes for:

- **Severity badges**: `severity-critical`, `severity-warning`, `severity-info`
- **Status badges**: `status-active`, `status-completed`, `status-failed`
- **Graph node colors**: Based on health status (green/yellow/red)

## Real-Time Updates

The dashboard polls the backend every 5 seconds for:

- New sessions appearing in the active list
- Session detail updates (new queries, findings, hypothesis changes)
- Blast zone graph updates
- Timeline events

Future enhancement: Replace polling with Server-Sent Events (SSE) or WebSocket notifications.

## File Structure

```
frontend/
├── src/
│   ├── components/        # React components
│   ├── services/          # MCP communication
│   ├── types/             # TypeScript types
│   ├── utils/             # Helper functions
│   ├── styles/            # CSS files
│   ├── App.tsx            # Main app
│   └── main.tsx           # Entry point
├── public/                # Static assets
├── index.html             # HTML template
├── package.json           # Dependencies
├── tsconfig.json          # TypeScript config
├── vite.config.ts         # Vite config
└── tailwind.config.js     # Tailwind config
```

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)

## License

Same as parent project.

