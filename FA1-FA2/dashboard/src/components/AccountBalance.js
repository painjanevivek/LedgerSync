import styles from '../styles/AccountBalance.module.css'

export default function AccountBalance({ accountId, balance, isStale, currency }) {
  return (
    <div className={styles.container}>
      <h3>Account: {accountId}</h3>
      <div className={styles.balance}>
        <span className={isStale ? styles.stale : ''}>
          {currency} {balance.toFixed(2)}
        </span>
        {isStale && (
          <span className={styles.staleLabel}>
            ( potentially stale )
          </span>
        )}
      </div>
      <div className={styles.info}>
        {isStale ? (
          <p>This balance might not reflect the most recent transaction.</p>
        ) : (
          <p>This balance is up-to-date with the latest transactions.</p>
        )}
      </div>
    </div>
  )
}