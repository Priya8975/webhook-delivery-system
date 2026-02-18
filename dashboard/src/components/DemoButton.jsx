import { useState } from 'react'
import { runDemo } from '../hooks/useApi'

export default function DemoButton() {
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState(null)

  async function handleClick() {
    setRunning(true)
    setResult(null)
    try {
      await runDemo()
      setResult('Demo events fired! Watch the live feed below.')
    } catch (err) {
      setResult(`Error: ${err.message}`)
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="flex items-center gap-3">
      <button
        onClick={handleClick}
        disabled={running}
        className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {running ? 'Running Demo...' : 'Run Demo'}
      </button>
      {result && (
        <span className="text-sm text-gray-500">{result}</span>
      )}
    </div>
  )
}
