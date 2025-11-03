export function EmptyState() {
  return (
    <div className="flex items-center justify-center h-full">
      <div className="text-center max-w-md">
        <svg
          className="mx-auto h-24 w-24 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
          />
        </svg>
        <h3 className="mt-4 text-lg font-medium text-gray-900">
          No Active Investigations
        </h3>
        <p className="mt-2 text-sm text-gray-500">
          Start an agent investigation session in Cursor to see it visualized here.
        </p>
        <div className="mt-6 text-xs text-gray-400">
          Use the <code className="bg-gray-100 px-2 py-1 rounded">start_agent_session</code> MCP tool
        </div>
      </div>
    </div>
  );
}

