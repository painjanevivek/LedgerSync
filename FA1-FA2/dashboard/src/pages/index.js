import Head from 'next/head'
import { useState, useEffect } from 'react'
import AccountBalance from '../components/AccountBalance'
import TransferForm from '../components/TransferForm'
import SimulationControls from '../components/SimulationControls'
import styles from '../styles/Home.module.css'

export default function Home() {
  const [accounts, setAccounts] = useState([])
  const [selectedAccount, setSelectedAccount] = useState(null)
  const [balance, setBalance] = useState(null)
  const [isStale, setIsStale] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [transferResult, setTransferResult] = useState(null)

  // Fetch accounts on mount
  useEffect(() => {
    fetchAccounts()
  }, [])

  const fetchAccounts = async () => {
    try {
      const response = await fetch('http://localhost:50051/v1/accounts')
      if (!response.ok) {
        throw new Error('Failed to fetch accounts')
      }
      const data = await response.json()
      setAccounts(data.accounts || [])
    } catch (err) {
      setError('Could not fetch accounts: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const fetchBalance = async (accountId) => {
    setLoading(true)
    try {
      const response = await fetch(`http://localhost:50051/v1/balance/${accountId}`)
      if (!response.ok) {
        throw new Error('Failed to fetch balance')
      }
      const data = await response.json()
      setBalance(data.balance)
      setIsStale(data.is_stale || false)
      setSelectedAccount(accountId)
    } catch (err) {
      setError('Could not fetch balance: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleTransfer = async (transferData) => {
    setLoading(true)
    try {
      const response = await fetch('http://localhost:50051/v1/transfer', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(transferData),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.message || 'Transfer failed')
      }

      const result = await response.json()
      setTransferResult(result)

      // Update balance if we're viewing one of the involved accounts
      if (selectedAccount === result.from_account.account_id ||
          selectedAccount === result.to_account.account_id) {
        await fetchBalance(selectedAccount)
      }
    } catch (err) {
      setError('Transfer failed: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return <div className={styles.container}>Loading...</div>
  }

  if (error) {
    return <div className={styles.container}>Error: {error}</div>
  }

  return (
    <div className={styles.container}>
      <Head>
        <title>FinTech Distributed System</title>
        <meta name="description" content="Real-time balance tracking with read-your-writes consistency" />
      </Head>

      <main>
        <h1>FinTech Distributed System Dashboard</h1>

        <div className={styles.grid}>
          <section className={styles.card}>
            <h2>Account Balances</h2>
            {accounts.length > 0 ? (
              <>
                <select
                  value={selectedAccount || ''}
                  onChange={(e) => fetchBalance(e.target.value)}
                  className={styles.select}
                >
                  <option value="">Select an account</option>
                  {accounts.map(acc => (
                    <option key={acc} value={acc}>{acc}</option>
                  ))}
                </select>

                {selectedAccount && balance !== null && (
                  <AccountBalance
                    accountId={selectedAccount}
                    balance={balance}
                    isStale={isStale}
                    currency="USD"
                  />
                )}
              </>
            ) : (
              <p>No accounts available</p>
            )}
          </section>

          <section className={styles.card}>
            <h2>Transfer Money</h2>
            <TransferForm
              accounts={accounts}
              onTransfer={handleTransfer}
              loading={loading}
            />
            {transferResult && (
              <div className={styles.result}>
                <h3>Transfer Result</h3>
                <p>{transferResult.message}</p>
                <p><strong>RYEW Token:</strong> {transferResult.ryew_token}</p>
              </div>
            )}
          </section>

          <section className={styles.card}>
            <h2>Simulation Controls</h2>
            <SimulationControls />
          </section>
        </div>
      </main>
    </div>
  )
}