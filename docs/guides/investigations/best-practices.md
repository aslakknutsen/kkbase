# Investigation Best Practices

Patterns, anti-patterns, and tips for effective autonomous diagnostics.

## Do's and Don'ts

### ✅ DO: Update Hypothesis Frequently

```
✅ Good: Update hypothesis each time understanding evolves
- Stage 1: "Service may be down"
- Stage 2: "Pods are crashing"  
- Stage 3: "OOM kills detected"
- Stage 4: "Memory leak in v2.3.5"

❌ Bad: Single hypothesis for entire investigation
- "Something is wrong with the service"
```

### ✅ DO: Provide Query Reasoning

```
✅ Good:
query_with_session(
  query="MATCH (s:Service {name: 'orders'})-[:SELECTS_PODS]->(p:Pod)",
  reasoning="Check health status of pods backing the orders service to identify any failed instances"
)

❌ Bad:
query_with_session(
  query="MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)",
  reasoning="check pods"
)
```

### ✅ DO: Record Multiple Recommendations

```
✅ Good: Multiple actionable recommendations
1. Root Cause Fix (Critical): Rollback to v2.3.4
2. Preventive Action (High): Increase memory limits
3. Monitoring (Medium): Add memory growth alerts
4. Cleanup (Low): Remove old deployments

❌ Bad: Single vague recommendation
"Fix the memory issue"
```

### ✅ DO: Link Recommendations to Findings

```
✅ Good:
record_recommendation(
  ...
  related_findings=["finding-oomkill-1", "finding-oomkill-2", "finding-deploy-1"]
)

❌ Bad: Recommendation without evidence
related_findings=[]
```

### ❌ DON'T: Skip Session Completion

```
❌ Bad: Leave session open
- Investigation stuck in "active" state
- Blast zone never finalized
- Dashboard shows incomplete

✅ Good: Always complete
complete_agent_session(summary="Identified memory leak...")
```

### ❌ DON'T: Treat Investigations as Separate Flows

```
❌ Bad: Think of spawn_investigation as starting a new investigation
"Now I'll start an investigation for metrics"

✅ Good: Investigations are sub-flows within agent session
"Now I'll spawn a metrics investigation within this session"
```

## Investigation Patterns

### Pattern: Service Outage

**Symptom**: Service returning errors

**Steps**:
1. Find service and pods
2. Check pod health status
3. Check recent events
4. Check recent deployments
5. Identify cause
6. Recommend fix

**Example**:
```
start_agent_session("Service orders-api returning 503")
→ query: find service and pods
→ finding: 3 pods CrashLoopBackOff
→ update hypothesis: "Pods crashing"
→ query: check events
→ finding: OOMKilled events
→ update hypothesis: "Memory exhaustion"
→ spawn_investigation: memory metrics
→ query: recent deployments
→ finding: v2.3.5 deployed 15 min ago
→ record_finding: "Root cause: memory leak in v2.3.5"
→ record_recommendation: "Rollback to v2.3.4"
→ complete_agent_session
```

### Pattern: Performance Degradation

**Symptom**: High latency

**Steps**:
1. Find affected service
2. Check dependencies (database, cache, etc.)
3. Spawn metrics investigation
4. Identify bottleneck
5. Recommend optimization

### Pattern: Cascade Failure

**Symptom**: Multiple services failing

**Steps**:
1. Identify common dependency
2. Trace dependency chain
3. Find root cause
4. Assess blast radius
5. Prioritize fixes

## Query Strategy

### Start Broad, Then Narrow

```
✅ Good: Progressive refinement
1. Find service
2. Find pods for service
3. Find pods with issues
4. Check events for those specific pods

❌ Bad: Too specific too early
1. Check pod nginx-abc123 events
   (What if the issue is elsewhere?)
```

### Use Filters Effectively

```
✅ Good: Filter at appropriate level
MATCH (s:Service {name: $service_name})-[:SELECTS_PODS]->(p:Pod)
WHERE p.status <> 'Running'

❌ Bad: Retrieve all then filter in code
MATCH (p:Pod)
# Then filter in application
```

### Leverage Relationships

```
✅ Good: Use graph structure
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)-[:SCHEDULED_ON]->(n:Node)

❌ Bad: Multiple separate queries
1. Get service pods
2. For each pod, get node
```

## Metrics Investigation Strategy

### When to Spawn Investigations

**DO spawn when**:
- OOM kills detected → Need memory metrics
- High CPU alerts → Need CPU metrics
- Latency issues → Need request metrics
- Network errors → Need connection metrics

**DON'T spawn when**:
- Simple config errors (check ConfigMap)
- Image pull failures (check events)
- RBAC issues (check permissions)

### Multiple Investigations Per Session

```
✅ Good: Spawn multiple as needed
session_id: "abc123"
├── investigation-1: memory metrics for pod-1
├── investigation-2: cpu metrics for pod-2
└── investigation-3: network metrics for service-x

❌ Bad: One investigation per session
"I can only check one thing"
```

## Finding Classification

### Severity Guidelines

**Critical**:
- Services down
- Data loss risk
- Security breaches
- OOM kills

**Warning**:
- Degraded performance
- Resource pressure
- Recent deployments
- Configuration drift

**Info**:
- Capacity headroom
- Optimization opportunities
- Informational context

### Finding Types

Use appropriate types:
- `unhealthy_pod` - Pods not running
- `failed_dependency` - Dependency unavailable
- `error_spike` - Increased error rate
- `resource_pressure` - Resource constraints
- `configuration_issue` - Config problems
- `deployment_change` - Recent changes
- `root_cause` - Identified root cause

## Recommendation Guidelines

### Actionable and Specific

```
✅ Good:
Title: "Increase memory limit for orders-api deployment"
Action Items:
1. Update deployment manifest: limits.memory from 512Mi to 1Gi
2. Update requests.memory to 768Mi
3. Apply: kubectl apply -f deployment.yaml
4. Monitor for 24 hours

❌ Bad:
Title: "Fix memory"
Action Items:
1. Increase memory
```

### Include Automation Hints

```
✅ Good:
automation_hint: |
  kubectl set resources deployment orders-api \
    --limits=memory=1Gi \
    --requests=memory=768Mi

❌ Bad:
automation_hint: "Use kubectl"
```

### Provide Rationale

```
✅ Good:
rationale: "Memory usage analysis shows consistent growth pattern reaching 500-520Mi before OOMKill. The v2.3.5 deployment increased baseline memory by 30% compared to v2.3.4."

❌ Bad:
rationale: "Pods need more memory"
```

## Common Mistakes

### 1. Forgetting to Complete Sessions

**Impact**: Unclosed sessions, confused dashboard

**Fix**: Always call `complete_agent_session`

### 2. Not Linking Recommendations to Findings

**Impact**: Recommendations lack evidence

**Fix**: Use `related_findings` array

### 3. Vague Hypotheses

**Impact**: Poor tracking, unclear reasoning

**Fix**: Be specific about current theory

### 4. Too Many Queries Without Hypothesis Updates

**Impact**: Lost thread, unclear progress

**Fix**: Update hypothesis as understanding evolves

### 5. Not Recording Synthesized Insights

**Impact**: Only automatic findings, missing analysis

**Fix**: Use `record_finding` for root causes

## Performance Tips

### Query Optimization

1. **Use parameters**: Prevent query planning overhead
2. **Limit results**: Always use LIMIT
3. **Filter early**: WHERE before traversal
4. **Avoid N+1**: Batch related queries

### Session Management

1. **Keep sessions focused**: One incident per session
2. **Complete promptly**: Don't leave open indefinitely
3. **Spawn investigations judiciously**: Only when metrics needed
4. **Clean up**: Complete investigations after use

## Learning from History

### Review Past Investigations

```cypher
// Find similar past investigations
MATCH (s:AgentSession)
WHERE s.initial_symptom CONTAINS 'OOMKilled'
  AND s.status = 'completed'
RETURN s.id, s.summary
ORDER BY s.created_at DESC
LIMIT 5
```

### Extract Patterns

Study successful investigations:
- What queries were effective?
- How did hypothesis evolve?
- What recommendations worked?
- How long did it take?

### Improve Prompts

Customize agent prompts based on:
- Your cluster architecture
- Common failure modes
- Team runbooks
- Historical patterns

## See Also

- [Workflow Guide](workflow.md) - Complete examples
- [Agent Sessions](agent-sessions.md) - Tool reference
- [Metrics RCA](metrics-rca.md) - Metrics investigation
- [Query Guide](../querying/) - Cypher patterns

