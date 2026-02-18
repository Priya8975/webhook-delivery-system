const stateColors = {
  closed: { bg: 'bg-green-100', text: 'text-green-800', label: 'Closed' },
  open: { bg: 'bg-red-100', text: 'text-red-800', label: 'Open' },
  'half-open': { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'Half-Open' },
}

export default function SubscriberHealth({ subscribers }) {
  if (subscribers.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow">
        <div className="px-5 py-3 border-b border-gray-100">
          <h2 className="text-lg font-semibold text-gray-800">Subscriber Health</h2>
        </div>
        <div className="px-5 py-8 text-center text-gray-400 text-sm">
          No subscribers registered yet.
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow">
      <div className="px-5 py-3 border-b border-gray-100">
        <h2 className="text-lg font-semibold text-gray-800">Subscriber Health</h2>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 border-b border-gray-100">
              <th className="px-5 py-2 font-medium">Name</th>
              <th className="px-5 py-2 font-medium">Endpoint</th>
              <th className="px-5 py-2 font-medium">Status</th>
              <th className="px-5 py-2 font-medium">Circuit Breaker</th>
              <th className="px-5 py-2 font-medium">Failures</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {subscribers.map((sub) => {
              const cbStyle = stateColors[sub.circuit_breaker.state] || stateColors.closed
              return (
                <tr key={sub.id} className="hover:bg-gray-50">
                  <td className="px-5 py-3 font-medium text-gray-800">{sub.name}</td>
                  <td className="px-5 py-3 text-gray-500 font-mono text-xs truncate max-w-48"
                      title={sub.endpoint_url}>
                    {sub.endpoint_url}
                  </td>
                  <td className="px-5 py-3">
                    <span className={`inline-block w-2 h-2 rounded-full mr-2 ${sub.is_active ? 'bg-green-500' : 'bg-gray-400'}`}></span>
                    {sub.is_active ? 'Active' : 'Inactive'}
                  </td>
                  <td className="px-5 py-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${cbStyle.bg} ${cbStyle.text}`}>
                      {cbStyle.label}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-gray-600">{sub.circuit_breaker.failures}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
