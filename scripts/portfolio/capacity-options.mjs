export function parseCapacityOptions(args) {
  const result={outboxBatchSize:10,capacitySteadySeconds:300,capacityRate:30},seen=new Set();
  for(const arg of args){const match=/^--(batch|steady|rate)=(\d+)$/.exec(arg);if(!match||seen.has(match[1]))throw new Error('Use bounded batch, steady and rate options once each');seen.add(match[1]);const value=Number(match[2]);if(match[1]==='batch'){if(![10,25,50].includes(value))throw new Error('Unsupported measured batch');result.outboxBatchSize=value;}else if(match[1]==='rate'){if(![10,20,30].includes(value))throw new Error('Unsupported bounded rate');result.capacityRate=value;}else{if(![10,300].includes(value))throw new Error('Unsupported bounded steady interval');result.capacitySteadySeconds=value;}}
  return result;
}
