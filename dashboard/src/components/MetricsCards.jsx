export default function MetricsCards({ metrics }) {
  if (!metrics) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="bg-white rounded-lg shadow p-5 animate-pulse">
            <div className="h-4 bg-gray-200 rounded w-24 mb-3"></div>
            <div className="h-8 bg-gray-200 rounded w-16"></div>
          </div>
        ))}
      </div>
    )
  }

  const cards = [
    {
      label: 'Total Delivered',
      value: metrics.total_deliveries,
      color: 'text-blue-600',
      bg: 'bg-blue-50',
    },
    {
      label: 'Success Rate',
      value: `${metrics.success_rate.toFixed(1)}%`,
      color: metrics.success_rate >= 90 ? 'text-green-600' : metrics.success_rate >= 50 ? 'text-yellow-600' : 'text-red-600',
      bg: metrics.success_rate >= 90 ? 'bg-green-50' : metrics.success_rate >= 50 ? 'bg-yellow-50' : 'bg-red-50',
    },
    {
      label: 'Queue Depth',
      value: metrics.queue_depth,
      color: 'text-purple-600',
      bg: 'bg-purple-50',
    },
    {
      label: 'Dead Letters',
      value: metrics.dead_letter_count,
      color: metrics.dead_letter_count > 0 ? 'text-red-600' : 'text-gray-600',
      bg: metrics.dead_letter_count > 0 ? 'bg-red-50' : 'bg-gray-50',
    },
  ]

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div key={card.label} className={`${card.bg} rounded-lg shadow p-5`}>
          <p className="text-sm text-gray-500 font-medium">{card.label}</p>
          <p className={`text-2xl font-bold mt-1 ${card.color}`}>{card.value}</p>
        </div>
      ))}
    </div>
  )
}
