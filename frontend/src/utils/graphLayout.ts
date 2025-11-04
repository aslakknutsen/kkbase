import dagre from 'dagre';
import type { Node, Edge } from 'reactflow';
import { MarkerType } from 'reactflow';
import type { BlastZoneNode, BlastZoneEdge } from '../types/blastZone';

// Node dimensions by type
const NODE_DIMENSIONS = {
  Namespace: { width: 400, height: 300 },
  Service: { width: 160, height: 70 },
  Deployment: { width: 140, height: 60 },
  ReplicaSet: { width: 140, height: 60 },
  Pod: { width: 120, height: 50 },
  default: { width: 140, height: 60 },
};

export interface LayoutOptions {
  showControllers?: boolean;
  groupByNamespace?: boolean;
  focusOnFailures?: boolean;
}

/**
 * Filter to show only failed nodes and their immediate neighbors (1 upstream, 1 downstream)
 */
function filterToFailureFocus(
  nodes: BlastZoneNode[],
  edges: BlastZoneEdge[]
): {
  nodes: BlastZoneNode[];
  edges: BlastZoneEdge[];
} {
  // Find all failed or degraded nodes
  const problemNodes = nodes.filter(
    (node) => node.status === 'failed' || node.status === 'degraded'
  );

  if (problemNodes.length === 0) {
    // No failed nodes, return all nodes
    return { nodes, edges };
  }

  const keepNodeIds = new Set<string>();
  const keepEdges: BlastZoneEdge[] = [];

  // For each problem node, find 1 upstream and 1 downstream
  problemNodes.forEach((problemNode) => {
    keepNodeIds.add(problemNode.id);

    // Find upstream nodes (nodes that point TO this problem node)
    const upstreamEdges = edges.filter((edge) => edge.target === problemNode.id);
    if (upstreamEdges.length > 0) {
      // Prioritize failing edges, then traffic edges, then any edge
      const upstreamEdge =
        upstreamEdges.find((e) => e.status === 'failing') ||
        upstreamEdges.find((e) =>
          ['CALLS', 'ROUTES_TO', 'SENDS_TO'].includes(e.type.toUpperCase())
        ) ||
        upstreamEdges[0];
      
      keepNodeIds.add(upstreamEdge.source);
      keepEdges.push(upstreamEdge);
    }

    // Find downstream nodes (nodes that this problem node points TO)
    const downstreamEdges = edges.filter((edge) => edge.source === problemNode.id);
    if (downstreamEdges.length > 0) {
      // Prioritize failing edges, then traffic edges, then any edge
      const downstreamEdge =
        downstreamEdges.find((e) => e.status === 'failing') ||
        downstreamEdges.find((e) =>
          ['CALLS', 'ROUTES_TO', 'SENDS_TO'].includes(e.type.toUpperCase())
        ) ||
        downstreamEdges[0];
      
      keepNodeIds.add(downstreamEdge.target);
      keepEdges.push(downstreamEdge);
    }
  });

  // Filter nodes to only those we want to keep
  const filteredNodes = nodes.filter((node) => keepNodeIds.has(node.id));

  return {
    nodes: filteredNodes,
    edges: keepEdges,
  };
}

export function layoutGraph(
  nodes: BlastZoneNode[], 
  edges: BlastZoneEdge[],
  options: LayoutOptions = {}
): {
  nodes: Node[];
  edges: Edge[];
} {
  const { showControllers = true, groupByNamespace = true, focusOnFailures = false } = options;

  // Filter nodes based on options
  let filteredNodes = nodes;
  let filteredEdges = edges;

  // Focus on failures: only show failed nodes + 1 upstream + 1 downstream
  if (focusOnFailures) {
    const focusedResult = filterToFailureFocus(nodes, edges);
    filteredNodes = focusedResult.nodes;
    filteredEdges = focusedResult.edges;
  }

  if (!showControllers) {
    // Hide controller nodes
    const controllerTypes = ['Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'ReplicationController'];
    filteredNodes = filteredNodes.filter(
      (node) => !controllerTypes.includes(node.type)
    );
  }

  // Group nodes by namespace
  const nodesByNamespace = new Map<string, BlastZoneNode[]>();
  const namespaceNodes: BlastZoneNode[] = [];

  filteredNodes.forEach((node) => {
    if (node.type === 'Namespace') {
      namespaceNodes.push(node);
    } else {
      const namespace = node.properties?.namespace || 'default';
      if (!nodesByNamespace.has(namespace)) {
        nodesByNamespace.set(namespace, []);
      }
      nodesByNamespace.get(namespace)!.push(node);
    }
  });

  // Filter edges based on filtered nodes and options
  const nodeIds = new Set(filteredNodes.map(n => n.id));
  filteredEdges = filteredEdges.filter(
    (edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target)
  );
  
  if (!showControllers) {
    // Also hide controller relationships
    filteredEdges = filteredEdges.filter(
      (edge) => !['OWNS', 'CONTROLS', 'MANAGES'].includes(edge.type.toUpperCase())
    );
  }

  const flowNodes: Node[] = [];
  const flowEdges: Edge[] = [];

  if (groupByNamespace && nodesByNamespace.size > 0) {
    // Layout with namespace grouping
    let namespaceY = 50;
    const namespaceSpacing = 100;
    const namespacePadding = 40;

    // Sort namespaces for stable layout
    const sortedNamespaces = Array.from(nodesByNamespace.entries()).sort(([a], [b]) => a.localeCompare(b));

    sortedNamespaces.forEach(([namespace, nsNodes]) => {
      // Skip empty namespaces
      if (nsNodes.length === 0) return;

      // Get node IDs in this namespace
      const nsNodeIds = new Set(nsNodes.map(n => n.id));

      // Get edges that are BOTH source AND target in this namespace
      const nsEdges = filteredEdges.filter(
        (e) => nsNodeIds.has(e.source) && nsNodeIds.has(e.target)
      );

      // Create namespace parent node
      const namespaceNodeId = `namespace-${namespace}`;
      let namespaceNode = namespaceNodes.find(
        (n) => n.label === namespace || n.id.endsWith(`/${namespace}`)
      );

      if (!namespaceNode) {
        namespaceNode = {
          id: namespaceNodeId,
          label: namespace,
          type: 'Namespace',
          status: 'healthy',
        };
      }

      // Layout nodes within namespace using hierarchical layout
      const { nodes: layoutedNodes, edges: layoutedEdges } = layoutHierarchical(
        nsNodes,
        nsEdges
      );

      // Adjust positions to be within namespace and relative to namespace container
      const offsetX = namespacePadding;
      const offsetY = namespaceY + namespacePadding + 30; // Extra space for label

      layoutedNodes.forEach((node) => {
        node.position.x += offsetX;
        node.position.y = offsetY + (node.position.y || 0);
        // DO NOT set parentNode - causes positioning issues
      });

      // Calculate bounds for namespace container
      const bounds = calculateBounds(layoutedNodes);

      // Add namespace container FIRST so it renders behind
      if (bounds.width > 0 && bounds.height > 0) {
        const containerWidth = bounds.width + namespacePadding * 2;
        const containerHeight = bounds.height + namespacePadding * 2 + 30;

        flowNodes.push({
          id: namespaceNode.id,
          type: 'group',
          position: { x: 0, y: namespaceY },
          data: {
            label: namespace,
            type: 'Namespace',
            status: namespaceNode.status,
          },
          style: {
            width: containerWidth,
            height: containerHeight,
            backgroundColor: 'rgba(241, 245, 249, 0.6)',
            border: '2px dashed #cbd5e1',
            borderRadius: 12,
            zIndex: -1,
          },
          selectable: false,
          draggable: false,
        } as Node);

        namespaceY += containerHeight + namespaceSpacing;
      }

      flowNodes.push(...layoutedNodes);
      flowEdges.push(...layoutedEdges);
    });

    // Add cross-namespace edges (edges where source and target are in different namespaces)
    filteredEdges.forEach((edge) => {
      // Check if this edge already exists
      const edgeExists = flowEdges.some(
        (e) => e.source === edge.source && e.target === edge.target
      );
      if (!edgeExists) {
        flowEdges.push(createFlowEdge(edge));
      }
    });
  } else {
    // Simple hierarchical layout without namespace grouping
    const { nodes: layoutedNodes, edges: layoutedEdges } = layoutHierarchical(
      filteredNodes,
      filteredEdges
    );
    flowNodes.push(...layoutedNodes);
    flowEdges.push(...layoutedEdges);
  }

  return { nodes: flowNodes, edges: flowEdges };
}

function layoutHierarchical(
  nodes: BlastZoneNode[],
  edges: BlastZoneEdge[]
): {
  nodes: Node[];
  edges: Edge[];
} {
  // Sort nodes for deterministic layout
  const sortedNodes = [...nodes].sort((a, b) => {
    // Sort by type first (Services, then Pods, then others)
    const typeOrder = { Service: 0, Pod: 1, Deployment: 2, ReplicaSet: 2, StatefulSet: 2, DaemonSet: 2 };
    const aOrder = typeOrder[a.type as keyof typeof typeOrder] ?? 99;
    const bOrder = typeOrder[b.type as keyof typeof typeOrder] ?? 99;
    if (aOrder !== bOrder) return aOrder - bOrder;
    
    // Then by label
    return a.label.localeCompare(b.label);
  });

  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  dagreGraph.setGraph({ 
    rankdir: 'LR',  // Left to right for better service flow visualization
    ranksep: 120,   // More space between layers
    nodesep: 60,    // Less vertical space
    marginx: 30,
    marginy: 30,
  });

  // Add nodes to dagre with appropriate dimensions
  sortedNodes.forEach((node) => {
    const dims = getNodeDimensions(node.type);
    dagreGraph.setNode(node.id, { width: dims.width, height: dims.height });
  });

  // Filter edges to only those with both endpoints in our node set
  const nodeIds = new Set(sortedNodes.map(n => n.id));
  const validEdges = edges.filter(
    (edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target)
  );

  // Add edges
  validEdges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  dagre.layout(dagreGraph);

  const flowNodes: Node[] = sortedNodes.map((node) => {
    const position = dagreGraph.node(node.id);
    const dims = getNodeDimensions(node.type);
    
    return {
      id: node.id,
      type: getNodeType(node.type),
      position: {
        x: position.x - dims.width / 2,
        y: position.y - dims.height / 2,
      },
      data: {
        label: node.label,
        type: node.type,
        status: node.status,
        properties: node.properties,
      },
      style: getNodeStyle(node.type, node.status),
    };
  });

  const flowEdges = validEdges.map((edge) => createFlowEdge(edge));

  return { nodes: flowNodes, edges: flowEdges };
}

function createFlowEdge(edge: BlastZoneEdge): Edge {
  const isTraffic = ['CALLS', 'ROUTES_TO', 'SENDS_TO'].includes(edge.type.toUpperCase());
  const isOwnership = ['OWNS', 'CONTROLS', 'MANAGES'].includes(edge.type.toUpperCase());
  const isFailing = edge.status === 'failing';
  
  return {
    id: `${edge.source}→${edge.target}`,
    source: edge.source,
    target: edge.target,
    label: edge.type,
    animated: isFailing && isTraffic,
    type: isTraffic ? 'default' : 'default', // Use default for all - smoothstep causes layout issues
    style: {
      stroke: isFailing ? '#ef4444' : isTraffic ? '#3b82f6' : '#94a3b8',
      strokeWidth: isFailing ? 3 : isTraffic ? 2 : 1,
      strokeDasharray: isOwnership ? '5,5' : undefined,
    },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: isFailing ? '#ef4444' : isTraffic ? '#3b82f6' : '#94a3b8',
    },
    labelStyle: {
      fontSize: 10,
      fill: '#64748b',
    },
    labelBgStyle: {
      fill: '#ffffff',
      fillOpacity: 0.8,
    },
  };
}

function getNodeType(resourceType: string): string {
  switch (resourceType) {
    case 'Service':
      return 'default';
    case 'Pod':
      return 'default';
    case 'Deployment':
    case 'ReplicaSet':
    case 'StatefulSet':
    case 'DaemonSet':
      return 'default';
    default:
      return 'default';
  }
}

function getNodeDimensions(type: string): { width: number; height: number } {
  return NODE_DIMENSIONS[type as keyof typeof NODE_DIMENSIONS] || NODE_DIMENSIONS.default;
}

function getNodeStyle(type: string, status: string): React.CSSProperties {
  const baseStyle: React.CSSProperties = {
    padding: 10,
    borderRadius: 8,
    border: '2px solid',
    fontSize: '12px',
    fontWeight: 500,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  };

  // Status colors
  let bgColor = '#d1fae5';
  let borderColor = '#10b981';
  let textColor = '#064e3b';

  if (status === 'failed') {
    bgColor = '#fecaca';
    borderColor = '#dc2626';
    textColor = '#7f1d1d';
  } else if (status === 'degraded') {
    bgColor = '#fef3c7';
    borderColor = '#f59e0b';
    textColor = '#78350f';
  }

  // Type-specific styles
  switch (type) {
    case 'Service':
      return {
        ...baseStyle,
        backgroundColor: status === 'failed' ? bgColor : '#dbeafe',
        borderColor: status === 'failed' ? borderColor : '#3b82f6',
        color: status === 'failed' ? textColor : '#1e3a8a',
        borderWidth: '3px',
      };
    case 'Pod':
      return {
        ...baseStyle,
        backgroundColor: bgColor,
        borderColor: borderColor,
        color: textColor,
        borderRadius: 6,
      };
    case 'Deployment':
    case 'ReplicaSet':
    case 'StatefulSet':
    case 'DaemonSet':
      return {
        ...baseStyle,
        backgroundColor: status === 'failed' ? bgColor : '#f3e8ff',
        borderColor: status === 'failed' ? borderColor : '#9333ea',
        color: status === 'failed' ? textColor : '#581c87',
        borderStyle: 'dashed',
      };
    default:
      return {
        ...baseStyle,
        backgroundColor: bgColor,
        borderColor: borderColor,
        color: textColor,
      };
  }
}

function calculateBounds(nodes: Node[]): {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  width: number;
  height: number;
} {
  if (nodes.length === 0) {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0 };
  }

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  nodes.forEach((node) => {
    const dims = getNodeDimensions(node.data.type);
    minX = Math.min(minX, node.position.x);
    minY = Math.min(minY, node.position.y);
    maxX = Math.max(maxX, node.position.x + dims.width);
    maxY = Math.max(maxY, node.position.y + dims.height);
  });

  return {
    minX,
    minY,
    maxX,
    maxY,
    width: maxX - minX,
    height: maxY - minY,
  };
}

