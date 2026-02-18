import MetricsCards from './components/MetricsCards'
import LiveFeed from './components/LiveFeed'
import SubscriberHealth from './components/SubscriberHealth'
import DeadLetterQueue from './components/DeadLetterQueue'
import DemoButton from './components/DemoButton'
import { useWebSocket } from './hooks/useWebSocket'
import { useMetrics, useSubscriberHealth, useDeadLetters } from './hooks/useApi'

function App() {
  const { events, connected, clearEvents } = useWebSocket()
  const { metrics } = useMetrics()
  const { subscribers } = useSubscriberHealth()
  const { deadLetters, resolve } = useDeadLetters()

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <div>
            <h1 className="text-xl font-bold text-gray-900">Webhook Delivery System</h1>
            <p className="text-sm text-gray-500">Real-time monitoring dashboard</p>
          </div>
          <DemoButton />
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-7xl mx-auto px-6 py-6 space-y-6">
        <MetricsCards metrics={metrics} />
        <LiveFeed events={events} connected={connected} onClear={clearEvents} />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <SubscriberHealth subscribers={subscribers} />
          <DeadLetterQueue deadLetters={deadLetters} onResolve={resolve} />
        </div>
      </main>
    </div>
  )
}

export default App
