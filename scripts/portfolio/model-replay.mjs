const kinds=new Set(['transfer','replay','conflict','fund','correct','lost-response','rollback','concurrent','insufficient']);

export function parseModelReplay(report) {
  const operations=report?.minimal_operations??report?.operations;
  if(!Number.isSafeInteger(report?.seed)||!Array.isArray(operations)||operations.length<1||operations.length>256)throw new Error('Invalid bounded model report');
  const selected=operations.map(op=>{
    if(!op||typeof op!=='object'||Object.keys(op).some(k=>!['kind','minor','reference','reverse'].includes(k))||!kinds.has(op.kind)||!Number.isSafeInteger(op.minor)||op.minor<0||op.minor>1000000||op.minor===0&&!['replay','conflict','correct'].includes(op.kind)||!Number.isSafeInteger(op.reference)||op.reference<0||op.reference>255||typeof op.reverse!=='boolean')throw new Error('Invalid bounded model operation');
    return {kind:op.kind,minor:op.minor,reference:op.reference,reverse:op.reverse};
  });
  const serialized=JSON.stringify({seed:report.seed,operations:selected});if(Buffer.byteLength(serialized)>24000)throw new Error('Model replay exceeds environment bound');return serialized;
}
