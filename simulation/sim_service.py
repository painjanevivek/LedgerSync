#!/usr/bin/env python3
"""
Simulation service for the FinTech distributed system.
This service simulates lag, stale reads, and invalidation to test the system.
"""

from flask import Flask, render_template, request, jsonify
import redis
import json
import time
import threading
import random
from datetime import datetime, timedelta

app = Flask(__name__)

# Redis connection
redis_client = redis.Redis(host='redis', port=6379, db=0, decode_responses=True)

# Simulation state
simulation_state = {
    'lag_enabled': False,
    'lag_duration': 0,  # seconds
    'stale_reads_enabled': False,
    'stale_read_probability': 0.0,  # 0.0 to 1.0
    'invalidation_delay': 0,  # seconds
    'manual_invalidation_accounts': [],  # accounts to manually invalidate
    'last_updated': datetime.now().isoformat()
}

# Store for simulated stale data
stale_data_store = {}


def get_account_balance_from_db(account_id):
    """Simulate getting balance from database"""
    # In a real system, this would query the actual database
    # For simulation, we return some deterministic values
    hash_val = hash(account_id) % 10000
    balance = 1000.0 + (hash_val % 9000)  # Between 1000 and 10000
    return round(balance, 2)


def get_account_balance_from_cache(account_id):
    """Get balance from Redis cache"""
    cached = redis_client.get(f"balance:{account_id}")
    if cached:
        return float(cached)
    return None


def set_account_balance_in_cache(account_id, balance):
    """Set balance in Redis cache"""
    redis_client.set(f"balance:{account_id}", str(balance))


def invalidate_account_balance(account_id):
    """Remove balance from cache (simulate invalidation)"""
    redis_client.delete(f"balance:{account_id}")


def add_to_invalidation_stream(account_ids):
    """Add invalidation event to Redis Stream"""
    invalidation_msg = {
        "account_ids": json.dumps(account_ids),
        "timestamp": str(int(time.time()))
    }
    redis_client.xadd("balance:invalidation", {"data": json.dumps(invalidation_msg)})


def background_lag_simulator():
    """Background thread to simulate lag in cache updates"""
    while True:
        if simulation_state['lag_enabled'] and simulation_state['lag_duration'] > 0:
            # In a real implementation, this would delay cache updates
            # For simulation, we'll just note that lag is active
            time.sleep(1)
        else:
            time.sleep(1)


def background_stale_read_simulator():
    """Background thread to simulate stale reads"""
    while True:
        if simulation_state['stale_reads_enabled'] and simulation_state['stale_read_probability'] > 0:
            # Randomly serve stale data for some requests
            # This is a simplified simulation
            time.sleep(1)
        else:
            time.sleep(1)


def background_invalidation_delayer():
    """Background thread to delay invalidation propagation"""
    while True:
        if simulation_state['invalidation_delay'] > 0:
            # Process manual invalidation requests with delay
            if simulation_state['manual_invalidation_accounts']:
                accounts_to_process = simulation_state['manual_invalidation_accounts'].copy()
                simulation_state['manual_invalidation_accounts'].clear()

                # Wait for the delay period
                time.sleep(simulation_state['invalidation_delay'])

                # Actually perform invalidation
                for account_id in accounts_to_process:
                    invalidate_account_balance(account_id)
                    add_to_invalidation_stream([account_id])

                # Update state
                simulation_state['last_updated'] = datetime.now().isoformat()
        else:
            # Process invalidation immediately
            if simulation_state['manual_invalidation_accounts']:
                accounts_to_process = simulation_state['manual_invalidation_accounts'].copy()
                simulation_state['manual_invalidation_accounts'].clear()

                for account_id in accounts_to_process:
                    invalidate_account_balance(account_id)
                    add_to_invalidation_stream([account_id])

                simulation_state['last_updated'] = datetime.now().isoformat()

        time.sleep(0.1)  # Check every 100ms


@app.route('/')
def index():
    """Serve the simulation dashboard"""
    return render_template('index.html', state=simulation_state)


@app.route('/api/state', methods=['GET'])
def get_state():
    """Get current simulation state"""
    return jsonify(simulation_state)


@app.route('/api/state', methods=['POST'])
def update_state():
    """Update simulation state"""
    global simulation_state
    data = request.get_json()

    # Update allowed fields
    for key in ['lag_enabled', 'lag_duration', 'stale_reads_enabled',
                'stale_read_probability', 'invalidation_delay']:
        if key in data:
            simulation_state[key] = data[key]

    simulation_state['last_updated'] = datetime.now().isoformat()
    return jsonify({'status': 'success', 'state': simulation_state})


@app.route('/api/invalidate', methods=['POST'])
def manual_invalidate():
    """Manually invalidate account balances"""
    data = request.get_json()
    account_ids = data.get('account_ids', [])

    if not account_ids:
        return jsonify({'error': 'No account IDs provided'}), 400

    if simulation_state['invalidation_delay'] > 0:
        # Add to queue for delayed processing
        simulation_state['manual_invalidation_accounts'].extend(account_ids)
    else:
        # Process immediately
        for account_id in account_ids:
            invalidate_account_balance(account_id)
            add_to_invalidation_stream([account_id])

    simulation_state['last_updated'] = datetime.now().isoformat()
    return jsonify({'status': 'success', 'invalidated': account_ids})


@app.route('/api/balance/<account_id>', methods=['GET'])
def get_balance(account_id):
    """Get balance for an account (with optional stale read simulation)"""
    # Simulate stale reads if enabled
    if (simulation_state['stale_reads_enabled'] and
        random.random() < simulation_state['stale_read_probability']):
        # Return potentially stale data
        balance = get_account_balance_from_cache(account_id)
        if balance is None:
            balance = get_account_balance_from_db(account_id)
        is_stale = True
    else:
        # Try cache first
        balance = get_account_balance_from_cache(account_id)
        is_stale = balance is None

        if balance is None:
            # Fallback to database
            balance = get_account_balance_from_db(account_id)
            # Update cache
            set_account_balance_in_cache(account_id, balance)

    return jsonify({
        'account_id': account_id,
        'balance': balance,
        'currency': 'USD',
        'is_stale': is_stale,
        'timestamp': datetime.now().isoformat()
    })


@app.route('/api/transfer', methods=['POST'])
def simulate_transfer():
    """Simulate a money transfer between accounts"""
    data = request.get_json()
    from_account = data.get('from_account')
    to_account = data.get('to_account')
    amount = data.get('amount')

    if not all([from_account, to_account, amount]):
        return jsonify({'error': 'Missing required parameters'}), 400

    try:
        amount = float(amount)
        if amount <= 0:
            return jsonify({'error': 'Amount must be positive'}), 400
    except ValueError:
        return jsonify({'error': 'Invalid amount'}), 400

    # Simulate getting balances
    from_balance = get_account_balance_from_db(from_account)
    to_balance = get_account_balance_from_db(to_account)

    if from_balance < amount:
        return jsonify({'error': 'Insufficient funds'}), 400

    # Simulate updating balances (in real system, this would be a DB transaction)
    new_from_balance = from_balance - amount
    new_to_balance = to_balance + amount

    # Update our simulated DB (in reality, this would be actual DB updates)
    # For simulation, we just update the cache to reflect new balances
    set_account_balance_in_cache(from_account, new_from_balance)
    set_account_balance_in_cache(to_account, new_to_balance)

    # Invalidate caches to force fresh reads
    invalidate_account_balance(from_account)
    invalidate_account_balance(to_account)
    add_to_invalidation_stream([from_account, to_account])

    # Generate a simple RYEW token (timestamp-based)
    ryew_token = f"{int(time.time())}-{random.randint(1000, 9999)}"

    simulation_state['last_updated'] = datetime.now().isoformat()

    return jsonify({
        'success': True,
        'message': 'Transfer successful',
        'ryew_token': ryew_token,
        'from_account': {
            'account_id': from_account,
            'old_balance': from_balance,
            'new_balance': new_from_balance
        },
        'to_account': {
            'account_id': to_account,
            'old_balance': to_balance,
            'new_balance': new_to_balance
        },
        'amount': amount
    })


if __name__ == '__main__':
    # Start background simulation threads
    lag_thread = threading.Thread(target=background_lag_simulator, daemon=True)
    stale_thread = threading.Thread(target=background_stale_read_simulator, daemon=True)
    invalidation_thread = threading.Thread(target=background_invalidation_delayer, daemon=True)

    lag_thread.start()
    stale_thread.start()
    invalidation_thread.start()

    # Initialize some sample account balances in cache
    sample_accounts = ['acc_001', 'acc_002', 'acc_003', 'acc_004', 'acc_005']
    for account in sample_accounts:
        balance = get_account_balance_from_db(account)
        set_account_balance_in_cache(account, balance)

    # Run the Flask app
    app.run(host='0.0.0.0', port=8000, debug=True)