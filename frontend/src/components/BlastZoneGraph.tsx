import { useEffect, useState } from 'react';
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  Panel,
  type NodeTypes,
} from 'reactflow';
import 'reactflow/dist/style.css';
import type { BlastZoneSnapshot } from '../types/blastZone';
import { layoutGraph, type LayoutOptions } from '../utils/graphLayout';

interface BlastZoneGraphProps {
  data: BlastZoneSnapshot | null;
}

// Custom Group Node component to show namespace labels
function GroupNode({ data }: { data: { label: string } }) {
  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <div
        style={{
          position: 'absolute',
          top: '8px',
          left: '12px',
          fontSize: '14px',
          fontWeight: 600,
          color: '#475569',
          backgroundColor: 'rgba(255, 255, 255, 0.9)',
          padding: '4px 12px',
          borderRadius: '6px',
          border: '1px solid #cbd5e1',
        }}
      >
        📦 {data.label}
      </div>
    </div>
  );
}

const nodeTypes: NodeTypes = {
  group: GroupNode,
};

export function BlastZoneGraph({ data }: BlastZoneGraphProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showControllers, setShowControllers] = useState(false);
  const [groupByNamespace, setGroupByNamespace] = useState(true);
  const [focusOnFailures, setFocusOnFailures] = useState(true);
  const [showLegend, setShowLegend] = useState(true);

  useEffect(() => {
    if (data && data.nodes && data.edges) {
      setIsLoading(false);
      const options: LayoutOptions = {
        showControllers,
        groupByNamespace,
        focusOnFailures,
      };
      const { nodes: layoutedNodes, edges: layoutedEdges } = layoutGraph(
        data.nodes,
        data.edges,
        options
      );
      setNodes(layoutedNodes);
      setEdges(layoutedEdges);
    } else {
      setIsLoading(true);
    }
  }, [data, showControllers, groupByNamespace, focusOnFailures, setNodes, setEdges]);

  if (isLoading || !data) {
    return (
      <div className="flex items-center justify-center h-[600px] bg-gray-50 rounded-lg">
        <div className="text-center">
          <svg
            className="animate-spin h-10 w-10 text-blue-500 mx-auto mb-3"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            ></circle>
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          <p className="text-sm text-gray-500">Calculating blast zone...</p>
        </div>
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-[600px] bg-gray-50 rounded-lg border-2 border-dashed border-gray-300">
        <div className="text-center">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"
            />
          </svg>
          <p className="mt-2 text-sm text-gray-500">
            No affected resources detected yet
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-[600px] border border-gray-200 rounded-lg overflow-hidden">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{
          padding: 0.2,
          duration: 0, // Disable animation to prevent jumping
        }}
        minZoom={0.1}
        maxZoom={2}
        attributionPosition="bottom-left"
        proOptions={{ hideAttribution: true }}
      >
        <Background />
        <Controls />
        <MiniMap
          nodeColor={(node) => {
            const status = node.data?.status;
            if (status === 'failed') return '#dc2626';
            if (status === 'degraded') return '#f59e0b';
            return '#10b981';
          }}
          maskColor="rgba(0, 0, 0, 0.1)"
        />
        
        {/* View Controls */}
        <Panel position="top-left" className="bg-white rounded-lg shadow-lg p-3 space-y-2">
          <div className="text-xs font-semibold text-gray-700 mb-2">View Options</div>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="checkbox"
              checked={focusOnFailures}
              onChange={(e) => setFocusOnFailures(e.target.checked)}
              className="rounded text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700">Focus on Failures</span>
          </label>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="checkbox"
              checked={groupByNamespace}
              onChange={(e) => setGroupByNamespace(e.target.checked)}
              className="rounded text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700">Group by Namespace</span>
          </label>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="checkbox"
              checked={showControllers}
              onChange={(e) => setShowControllers(e.target.checked)}
              className="rounded text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700">Show Controllers</span>
          </label>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="checkbox"
              checked={showLegend}
              onChange={(e) => setShowLegend(e.target.checked)}
              className="rounded text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700">Show Legend</span>
          </label>
        </Panel>

        {/* Legend */}
        {showLegend && (
          <Panel position="top-right" className="bg-white rounded-lg shadow-lg p-3">
            <div className="text-xs font-semibold text-gray-700 mb-2">Legend</div>
            <div className="space-y-2 text-xs">
              {/* Node Types */}
              <div>
                <div className="font-medium text-gray-600 mb-1">Resource Types</div>
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-blue-100 border-2 border-blue-600 rounded"></div>
                    <span className="text-gray-600">Service</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-purple-100 border-2 border-purple-600 rounded" style={{ borderStyle: 'dashed' }}></div>
                    <span className="text-gray-600">Controller</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-green-100 border-2 border-green-600 rounded"></div>
                    <span className="text-gray-600">Pod</span>
                  </div>
                </div>
              </div>

              {/* Status */}
              <div>
                <div className="font-medium text-gray-600 mb-1 mt-2">Status</div>
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-green-200 border-2 border-green-600 rounded"></div>
                    <span className="text-gray-600">Healthy</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-yellow-200 border-2 border-yellow-600 rounded"></div>
                    <span className="text-gray-600">Degraded</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <div className="w-12 h-6 bg-red-200 border-2 border-red-600 rounded"></div>
                    <span className="text-gray-600">Failed</span>
                  </div>
                </div>
              </div>

              {/* Edge Types */}
              <div>
                <div className="font-medium text-gray-600 mb-1 mt-2">Relationships</div>
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <svg width="40" height="10">
                      <line x1="0" y1="5" x2="40" y2="5" stroke="#3b82f6" strokeWidth="2" />
                    </svg>
                    <span className="text-gray-600">Traffic</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <svg width="40" height="10">
                      <line x1="0" y1="5" x2="40" y2="5" stroke="#94a3b8" strokeWidth="1" strokeDasharray="5,5" />
                    </svg>
                    <span className="text-gray-600">Ownership</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <svg width="40" height="10">
                      <line x1="0" y1="5" x2="40" y2="5" stroke="#ef4444" strokeWidth="3" />
                    </svg>
                    <span className="text-gray-600">Failing</span>
                  </div>
                </div>
              </div>
            </div>
          </Panel>
        )}
      </ReactFlow>
    </div>
  );
}

