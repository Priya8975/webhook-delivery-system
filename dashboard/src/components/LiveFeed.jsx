const typeStyles = {
  delivery_success: { label: 'SUCCESS', bg: 'bg-green-100', text: 'text-green-800' },
  delivery_retrying: { label: 'RETRY', bg: 'bg-yellow-100', text: 'text-yellow-800' },
  delivery_failed: { label: 'FAILED', bg: 'bg-red-100', text: 'text-red-800' },
  delivery_dlq: { label: 'DLQ', bg: 'bg-red-200', text: 'text-red-900' },
}

function formatTime(timestamp) {
  return new Date(timestamp).toLocaleTimeString()
}

function truncate(str, len) {
  if (!str) return ''
  return str.length > len ? str.slice(0, len) + '...' : str
}

export default function LiveFeed({ events, connected, onClear }) {
  return (
    <div className="bg-white rounded-lg shadow">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-100">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-semibold text-gray-800">Live Delivery Feed</h2>
          <span className={`inline-block w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}></span>
          <span className="text-xs text-gray-400">{connected ? 'Connected' : 'Disconnected'}</span>
        </div>
        <button
          onClick={onClear}
          className="text-xs text-gray-400 hover:text-gray-600 px-2 py-1 rounded hover:bg-gray-100"
        >
          Clear
        </button>
      </div>

      <div className="divide-y divide-gray-50 max-h-96 overflow-y-auto">
        {events.length === 0 ? (
          <div className="px-5 py-8 text-center text-gray-400 text-sm">
            Waiting for delivery events...
          </div>
        ) : (
          events.map((event, i) => {
            const style = typeStyles[event.type] || typeStyles.delivery_failed
            return (
              <div key={`${event.event_id}-${event.attempt}-${i}`} className="px-5 py-3 flex items-center gap-3 text-sm hover:bg-gray-50">
                <span className={`px-2 py-0.5 rounded text-xs font-medium ${style.bg} ${style.text}`}>
                  {style.label}
                </span>
                <span className="text-gray-500 w-20 shrink-0">{formatTime(event.timestamp)}</span>
                <span className="text-gray-700 font-mono text-xs truncate" title={event.event_id}>
                  {truncate(event.event_id, 8)}
                </span>
                <span className="text-gray-400">→</span>
                <span className="text-gray-600 truncate" title={event.endpoint_url}>
                  {truncate(event.endpoint_url, 35)}
                </span>
                <span className="ml-auto text-gray-400 text-xs shrink-0">
                  {event.response_ms}ms
                  {event.attempt > 1 && ` (attempt ${event.attempt})`}
                </span>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
