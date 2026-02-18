function truncate(str, len) {
  if (!str) return ''
  return str.length > len ? str.slice(0, len) + '...' : str
}

export default function DeadLetterQueue({ deadLetters, onResolve }) {
  return (
    <div className="bg-white rounded-lg shadow">
      <div className="px-5 py-3 border-b border-gray-100">
        <h2 className="text-lg font-semibold text-gray-800">Dead Letter Queue</h2>
      </div>

      {deadLetters.length === 0 ? (
        <div className="px-5 py-8 text-center text-gray-400 text-sm">
          No dead letters. All deliveries are healthy.
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b border-gray-100">
                <th className="px-5 py-2 font-medium">Event ID</th>
                <th className="px-5 py-2 font-medium">Subscriber</th>
                <th className="px-5 py-2 font-medium">Attempts</th>
                <th className="px-5 py-2 font-medium">Last Status</th>
                <th className="px-5 py-2 font-medium">Error</th>
                <th className="px-5 py-2 font-medium">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {deadLetters.map((dl) => (
                <tr key={dl.id} className="hover:bg-gray-50">
                  <td className="px-5 py-3 font-mono text-xs text-gray-600" title={dl.event_id}>
                    {truncate(dl.event_id, 8)}
                  </td>
                  <td className="px-5 py-3 font-mono text-xs text-gray-600" title={dl.subscriber_id}>
                    {truncate(dl.subscriber_id, 8)}
                  </td>
                  <td className="px-5 py-3 text-gray-600">{dl.total_attempts}</td>
                  <td className="px-5 py-3">
                    {dl.last_http_status ? (
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">
                        {dl.last_http_status}
                      </span>
                    ) : (
                      <span className="text-gray-400">—</span>
                    )}
                  </td>
                  <td className="px-5 py-3 text-gray-500 text-xs truncate max-w-40" title={dl.last_error}>
                    {dl.last_error || '—'}
                  </td>
                  <td className="px-5 py-3">
                    <button
                      onClick={() => onResolve(dl.id)}
                      className="text-xs px-3 py-1 rounded bg-blue-50 text-blue-700 hover:bg-blue-100 font-medium"
                    >
                      Resolve
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
