import { readdir, stat } from "node:fs/promises";
import { join } from "node:path";

const root = join(process.cwd(), ".next", "static", "chunks");
const limits = { largestChunkBytes: 350_000, totalChunkBytes: 2_000_000 };
const files = [];
async function walk(directory) { for (const name of await readdir(directory)) { const path=join(directory,name); const info=await stat(path); if(info.isDirectory()) await walk(path); else if(name.endsWith(".js")) files.push({path,bytes:info.size}); } }
await walk(root);
const total=files.reduce((sum,file)=>sum+file.bytes,0); const largest=files.sort((a,b)=>b.bytes-a.bytes)[0];
console.log(JSON.stringify({javascript_chunks:files.length,total_bytes:total,largest_chunk_bytes:largest?.bytes??0,largest_chunk:largest?.path,limits},null,2));
if(total>limits.totalChunkBytes||((largest?.bytes??0)>limits.largestChunkBytes)){console.error("Frontend JavaScript performance budget exceeded");process.exitCode=1;}
