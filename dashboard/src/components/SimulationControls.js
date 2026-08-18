import { useState, useEffect } from 'react'
import styles from '../styles/SimulationControls.module.css'

export default function SimulationControls() {
  const [state, setState] = useState({
    lag_enabled: false,
    lag_duration: 0,
    stale_reads_enabled: false,
    stale_read_probability: 0,
    invalidation_delay: 0
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  // Fetch initial state on mount
  useEffect(() => {
    fetchState()
  }, [])

  const fetchState = async () => {
    setLoading(true)
    try {
      const response = await fetch('http://localhost:8000/api/state')
      if (!response.ok) {
        throw new Error('Failed to fetch simulation state')
      }
      const data = await response.json()
      setState(data)
    } catch (err) {
      setError('Could not fetch simulation state: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const updateState = async (updates) => {
    setLoading(true)
    try {
      const response = await fetch('http://localhost:8000/api/state', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(updates),
      })

      if (!response.ok) {
        throw new Error('Failed to update simulation state')
      }

      const data = await response.json()
      setState(data.state)
    } catch (err) {
      setError('Failed to update state: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleManualInvalidate = async () => {
    setLoading(true)
    try {
      const response = await fetch('http://localhost:8000/api/invalidate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          account_ids: ['acc_001', 'acc_002', 'acc_003'] // Example accounts
        }),
      })

      if (!response.ok) {
        throw new Error('Failed to trigger manual invalidation')
      }

      const data = await response.json()
      // Optionally show success message
      console.log('Manual invalidation triggered:', data)
    } catch (err) {
      setError('Manual invalidation failed: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return <div className={styles.container}>Loading simulation controls...</div>
  }

  if (error) {
    return <div className={styles.container}>Error: {error}</div>
  }

  return (
    <div className={styles.container}>
      <h2>Simulation Controls</h2>
      <p>Use these controls to simulate network lag, stale reads, and delayed invalidation to test the read-your-writes consistency mechanism.</p>

      <div className={styles.controlGroup}>
        <h3>Network Lag Simulation</h3>
        <div className={styles.controlItem}>
          <label>
            <input
              type="checkbox"
              checked={state.lag_enabled}
              onChange={(e) => updateState({ lag_enabled: e.target.checked })}
              disabled={loading}
            />
            Enable Lag Simulation
          </label>
        </div>
        <div className={styles.controlItem}>
          <label htmlFor="lagDuration">Lag Duration (seconds):</label>
          <input
            id="lagDuration"
            type="number"
            value={state.lag_duration}
            onChange={(e) => updateState({ lag_duration: parseFloat(e.target.value) || 0 })}
            min="0"
            max="10"
            step="0.1"
            disabled={loading || !state.lag_enabled}
          />
        </div>
      </div>

      <div className={styles.controlGroup}>
        <h3>Stale Read Simulation</h3>
        <div className={styles.controlItem}>
          <label>
            <input
              type="checkbox"
              checked={state.stale_reads_enabled}
              onChange={(e) => updateState({ stale_reads_enabled: e.target.checked })}
              disabled={loading}
            />
            Enable Stale Reads
          </label>
        </div>
        <div className={styles.controlItem}>
          <label htmlFor="staleProbability">Stale Read Probability:</label>
          <input
            id="staleProbability"
            type="number"
            value={state.stale_read_probability}
            onChange={(e) => updateState({ stale_read_probability: parseFloat(e.target.value) || 0 })}
            min="0"
            max="1"
            step="0.1"
            disabled={loading || !state.stale_reads_enabled}
          />
          <span className={styles.helpText}>
            (0.0 = never, 1.0 = always)
          </span>
        </div>
      </div>

      <div className={styles.controlGroup}>
        <h3>Invalidation Delay Simulation</h3>
        <div className={styles.controlItem}>
          <label htmlFor="invalidationDelay">Invalidation Delay (seconds):</label>
          <input
            id="invalidationDelay"
            type="number"
            value={state.invalidation_delay}
            onChange={(e) => updateState({ invalidation_delay: parseFloat(e.target.value) || 0 })}
            min="0"
            max="10"
            step="0.1"
            disabled={loading}
          />
          <span className={styles.helpText}>
            Delay before cache invalidation propagates (simulates network propagation delay)
          </span>
        </div>
      </div>

      <div className={styles.controlGroup}>
        <h3>Manual Controls</h2>
        <button
          className={styles.button}
          onClick={handleManualInvalidate}
          disabled={loading}
        >
          Trigger Manual Invalidation
        </button>
      </div>

      <div className={styles.statusPanel}>
        <h3>Current Status</h3>
        <div className={styles.statusItem}>
          <span>Lag: </span>
          <span className={state.lag_enabled ? styles.active : styles.inactive}>
            {state.lag_enabled ? 'ON' : 'OFF'}
          </span>
        </div>
        <div className={styles.statusItem}>
          <span>Stale Reads: </span>
          <span className={state.stale_reads_enabled ? styles.active : styles.inactive}>
            {state.stale_reads_enabled ? 'ON' : 'OFF'}
          </span>
        </div>
        <div className={styles.statusItem}>
          <span>Invalidation Delay: </span>
          <span>{state.invalidation_delay}s</span>
        </div>
      </div>
    </div>
  )
}