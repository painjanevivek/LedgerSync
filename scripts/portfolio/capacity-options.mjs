export const capacityProfiles = Object.freeze(['mixed', 'independent-pairs', 'hot-pair', 'multi-tenant']);
export function parseCapacityOptions(args) {
  const result = { outboxBatchSize: 10, capacitySteadySeconds: 300, capacityRate: 30, capacityProfile: 'mixed', capacityArrivalMode: 'open-loop' };
  const seen = new Set();
  for (const arg of args) {
    const match = /^--(batch|steady|rate|profile|arrival)=([a-z0-9-]+)$/.exec(arg);
    if (!match || seen.has(match[1])) throw Error('Use each bounded capacity option at most once');
    const [, key, raw] = match;
    seen.add(key);
    if (key === 'profile') {
      if (!capacityProfiles.includes(raw)) throw Error('Unsupported fixed workload profile');
      result.capacityProfile = raw;
    } else if (key === 'arrival') {
      if (!['open-loop', 'closed-clients'].includes(raw)) throw Error('Unsupported arrival model');
      result.capacityArrivalMode = raw;
    } else {
      if (!/^\d+$/.test(raw)) throw Error('Canonical positive numeric option required');
      const value = Number(raw);
      const definitions = { batch: [[10, 25, 50], 'outboxBatchSize'], rate: [[10, 20, 30], 'capacityRate'], steady: [[10, 300], 'capacitySteadySeconds'] };
      const [allowed, property] = definitions[key];
      if (!allowed.includes(value)) throw Error('Unsupported bounded capacity value');
      result[property] = value;
    }
  }
  return result;
}
