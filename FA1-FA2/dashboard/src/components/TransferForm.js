import { useState } from 'react'
import styles from '../styles/TransferForm.module.css'

export default function TransferForm({ accounts, onTransfer, loading }) {
  const [formData, setFormData] = useState({
    fromAccount: '',
    toAccount: '',
    amount: ''
  })
  const [submitError, setSubmitError] = useState(null)

  const handleChange = (e) => {
    const { name, value } = e.target
    setFormData(prev => ({
      ...prev,
      [name]: value
    }))
    setSubmitError(null)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setSubmitError(null)

    // Basic validation
    if (!formData.fromAccount || !formData.toAccount || !formData.amount) {
      setSubmitError('Please fill in all fields')
      return
    }

    if (formData.fromAccount === formData.toAccount) {
      setSubmitError('From and To accounts must be different')
      return
    }

    const amount = parseFloat(formData.amount)
    if (isNaN(amount) || amount <= 0) {
      setSubmitError('Amount must be a positive number')
      return
    }

    try {
      await onTransfer({
        from_account: formData.fromAccount,
        to_account: formData.toAccount,
        amount: amount
      })

      // Reset form on successful transfer
      setFormData({
        fromAccount: '',
        toAccount: '',
        amount: ''
      })
    } catch (error) {
      setSubmitError(error.message || 'Transfer failed')
    }
  }

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <div className={styles.formGroup}>
        <label htmlFor="fromAccount">From Account:</label>
        <select
          id="fromAccount"
          name="fromAccount"
          value={formData.fromAccount}
          onChange={handleChange}
          className={styles.select}
          disabled={loading}
        >
          <option value="">Select account</option>
          {accounts.map(account => (
            <option key={account} value={account}>{account}</option>
          ))}
        </select>
      </div>

      <div className={styles.formGroup}>
        <label htmlFor="toAccount">To Account:</label>
        <select
          id="toAccount"
          name="toAccount"
          value={formData.toAccount}
          onChange={handleChange}
          className={styles.select}
          disabled={loading}
        >
          <option value="">Select account</option>
          {accounts.map(account => (
            <option key={account} value={account}>{account}</option>
          ))}
        </select>
      </div>

      <div className={styles.formGroup}>
        <label htmlFor="amount">Amount ($):</label>
        <input
          id="amount"
          name="amount"
          type="number"
          value={formData.amount}
          onChange={handleChange}
          className={styles.input}
          step="0.01"
          min="0.01"
          disabled={loading}
        />
      </div>

      {submitError && (
        <div className={styles.error}>
          {submitError}
        </div>
      )}

      <button
        type="submit"
        className={styles.button}
        disabled={loading}
      >
        {loading ? 'Processing...' : 'Transfer Money'}
      </button>
    </form>
  )
}