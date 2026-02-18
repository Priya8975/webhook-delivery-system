import { useState, useEffect, useCallback } from 'react'

const API_BASE = '/api/v1'

async function fetchJSON(url) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

export function useMetrics(refreshInterval = 2000) {
  const [metrics, setMetrics] = useState(null)
  const [error, setError] = useState(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchJSON(`${API_BASE}/metrics`)
      setMetrics(data)
      setError(null)
    } catch (err) {
      setError(err.message)
    }
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, refreshInterval)
    return () => clearInterval(interval)
  }, [refresh, refreshInterval])

  return { metrics, error, refresh }
}

export function useSubscriberHealth(refreshInterval = 3000) {
  const [subscribers, setSubscribers] = useState([])
  const [error, setError] = useState(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchJSON(`${API_BASE}/subscribers-health`)
      setSubscribers(data)
      setError(null)
    } catch (err) {
      setError(err.message)
    }
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, refreshInterval)
    return () => clearInterval(interval)
  }, [refresh, refreshInterval])

  return { subscribers, error, refresh }
}

export function useDeadLetters(refreshInterval = 5000) {
  const [deadLetters, setDeadLetters] = useState([])
  const [error, setError] = useState(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchJSON(`${API_BASE}/dead-letters`)
      setDeadLetters(data)
      setError(null)
    } catch (err) {
      setError(err.message)
    }
  }, [])

  const resolve = useCallback(async (id) => {
    try {
      await fetch(`${API_BASE}/dead-letters/${id}/resolve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ resolved_by: 'dashboard' }),
      })
      refresh()
    } catch (err) {
      setError(err.message)
    }
  }, [refresh])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, refreshInterval)
    return () => clearInterval(interval)
  }, [refresh, refreshInterval])

  return { deadLetters, error, refresh, resolve }
}

export async function runDemo() {
  // Step 1: Create a test subscriber pointing to mock endpoint
  const subRes = await fetch(`${API_BASE}/subscribers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: `Demo Subscriber ${Date.now().toString(36)}`,
      endpoint_url: 'http://localhost:9090/webhook/success',
      event_types: ['demo.event'],
      rate_limit_per_second: 10,
    }),
  })
  const subscriber = await subRes.json()

  // Step 2: Create a failing subscriber to demonstrate retries + DLQ
  await fetch(`${API_BASE}/subscribers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: `Demo Flaky ${Date.now().toString(36)}`,
      endpoint_url: 'http://localhost:9090/webhook/flaky',
      event_types: ['demo.event'],
      rate_limit_per_second: 10,
    }),
  })

  // Step 3: Fire off a few events
  for (let i = 0; i < 3; i++) {
    await fetch(`${API_BASE}/events`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        event_type: 'demo.event',
        payload: {
          message: `Demo event #${i + 1}`,
          demo: true,
          timestamp: new Date().toISOString(),
        },
      }),
    })
  }

  return subscriber
}
