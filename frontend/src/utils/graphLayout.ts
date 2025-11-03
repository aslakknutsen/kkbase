import dagre from 'dagre';
import type { Node, Edge } from 'reactflow';
import { MarkerType } from 'reactflow';
import type { BlastZoneNode, BlastZoneEdge } from '../types/blastZone';

const nodeWidth = 180;
const nodeHeight = 60;

export function layoutGraph(nodes: BlastZoneNode[], edges: BlastZoneEdge[]): {
  nodes: Node[];
  edges: Edge[];
} {
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  dagreGraph.setGraph({ rankdir: 'TB', ranksep: 80, nodesep: 100 });

  // Add nodes to dagre
  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: nodeWidth, height: nodeHeight });
  });

  // Add edges to dagre
  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  // Calculate layout
  dagre.layout(dagreGraph);

  // Convert to React Flow format
  const flowNodes: Node[] = nodes.map((node) => {
    const position = dagreGraph.node(node.id);
    
    return {
      id: node.id,
      type: 'default',
      position: {
        x: position.x - nodeWidth / 2,
        y: position.y - nodeHeight / 2,
      },
      data: {
        label: node.label,
        type: node.type,
        status: node.status,
        properties: node.properties,
      },
      style: getNodeStyle(node.status),
    };
  });

  const flowEdges: Edge[] = edges.map((edge) => ({
    id: `${edge.source}-${edge.target}`,
    source: edge.source,
    target: edge.target,
    label: edge.type,
    animated: edge.status === 'failing',
    style: {
      stroke: edge.status === 'failing' ? '#ef4444' : '#94a3b8',
      strokeWidth: edge.status === 'failing' ? 2 : 1,
    },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: edge.status === 'failing' ? '#ef4444' : '#94a3b8',
    },
  }));

  return { nodes: flowNodes, edges: flowEdges };
}

function getNodeStyle(status: string): React.CSSProperties {
  const baseStyle: React.CSSProperties = {
    padding: 10,
    borderRadius: 8,
    border: '2px solid',
    fontSize: '12px',
    fontWeight: 500,
  };

  switch (status) {
    case 'failed':
      return {
        ...baseStyle,
        backgroundColor: '#fecaca',
        borderColor: '#dc2626',
        color: '#7f1d1d',
      };
    case 'degraded':
      return {
        ...baseStyle,
        backgroundColor: '#fef3c7',
        borderColor: '#f59e0b',
        color: '#78350f',
      };
    default:
      return {
        ...baseStyle,
        backgroundColor: '#d1fae5',
        borderColor: '#10b981',
        color: '#064e3b',
      };
  }
}

