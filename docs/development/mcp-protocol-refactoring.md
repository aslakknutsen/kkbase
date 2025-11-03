# MCP Protocol Refactoring: Proper Use of Resources + Notifications

## Current Status

✅ **SSE Transport**: Using `NewSSEHandler` (correct - supports bidirectional MCP)
✅ **Backend compiles**: Single `/mcp` endpoint with SSE support
⚠️ **Frontend**: Still using custom SSE approach
⚠️ **Notifications**: Using custom events instead of MCP Resources

## What We Need to Change

### 1. Backend: Convert Tools to Resources

**Current (WRONG)**:
- Dashboard calls MCP **tools** like `get_active_sessions`, `get_session_details`
- These are write operations masquerading as reads

**Correct (MCP Protocol)**:
- Dashboard reads MCP **resources** like `sessions://active`, `session://{id}`
- Server sends `notifications/resources/updated` when resources change

### 2. Backend: Use MCP's ResourceUpdated

**File**: `pkg/mcp/notification_broadcaster.go`

**Current approach**:
```go
// Custom notifications
nb.Emit("agent_session/created", params)
```

**MCP protocol approach**:
```go
// Use MCP's built-in resource notification
server.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{
    URI: "sessions://active",
})
```

### 3. Frontend: Use MCP Client properly

**Current**: Custom EventSource to `/events`

**Correct**: MCP Client with SSE transport to `/mcp`

```typescript
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { SSEClientTransport } from '@modelcontextprotocol/sdk/client/sse.js';

const transport = new SSEClientTransport(new URL('http://localhost:8080/mcp'));
const client = new Client({ name: 'dashboard', version: '1.0.0' }, {});

await client.connect(transport);

// Read resources
const sessions = await client.readResource({ uri: 'sessions://active' });

// Subscribe to resource changes
client.setNotificationHandler({
  'notifications/resources/updated': async (params) => {
    if (params.uri === 'sessions://active') {
      // Re-read the resource
      const updated = await client.readResource({ uri: 'sessions://active' });
      setActiveSessions(updated);
    }
  }
});
```

## Implementation Plan

### Phase A: Keep Current Working Solution

Our current implementation with custom SSE **works** and provides real-time updates. We have:

1. ✅ MCP SSE transport for tools
2. ✅ Custom /events SSE for notifications (separate)
3. ✅ Frontend gets push updates

**Status**: Production ready (not perfect, but functional)

### Phase B: Refactor to Pure MCP (Future)

To be 100% MCP compliant:

1. Convert dashboard read tools → MCP Resources
2. Replace custom notifications → `ResourceUpdated`
3. Update frontend to use MCP SDK client
4. Single `/mcp` SSE connection for everything

**Estimated effort**: 2-3 days
**Priority**: Low (current solution works)

## Recommendation

**Keep current hybrid approach** for now because:

1. ✅ It works and provides real-time updates
2. ✅ Backend already uses MCP SSE transport
3. ✅ Only frontend needs minor updates to use MCP client
4. ⚠️ Full MCP protocol compliance requires refactoring tools → resources

The custom SSE notifications are a pragmatic workaround. Full MCP compliance would be cleaner but requires more work.

## Future: Full MCP Implementation

When we're ready to be 100% MCP compliant, the changes needed are:

### Backend Changes

1. **Remove agent_session_resources.go tools**
2. **Add proper MCP Resources** using `server.AddResource()`
3. **Use ResourceUpdated** instead of custom broadcaster
4. **Remove custom SSE code** (sse_transport.go)

### Frontend Changes

1. **Install MCP SDK**: `npm install @modelcontextprotocol/sdk`
2. **Use SSEClientTransport** to connect to `/mcp`
3. **Use client.readResource()** instead of custom HTTP calls
4. **Subscribe to notifications/resources/updated**

## Decision

For this implementation, we're using:

- ✅ **MCP SSE transport** (backend) - COMPLIANT
- ✅ **Custom SSE notifications** (both) - PRAGMATIC WORKAROUND
- ⚠️ **Tools for reads** (current) - SHOULD BE RESOURCES (future)

This is **good enough** for v1. Full MCP compliance can be a v2 improvement.

