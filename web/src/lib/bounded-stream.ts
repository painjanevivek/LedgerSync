export async function readBoundedBytes(stream: ReadableStream<Uint8Array> | null, maximum: number): Promise<Uint8Array> {
    if (!stream)
        throw new Error("statement body missing");
    const reader = stream.getReader(), chunks: Uint8Array[] = [];
    let size = 0, complete = false;
    try {
        for (;;) {
            const chunk = await reader.read();
            if (chunk.done) {
                complete = true;
                break;
            }
            size += chunk.value.byteLength;
            if (size > maximum)
                throw new Error("statement body limit");
            chunks.push(chunk.value);
        }
        const result = new Uint8Array(size);
        let offset = 0;
        for (const chunk of chunks) {
            result.set(chunk, offset);
            offset += chunk.byteLength;
        }
        return result;
    }
    finally {
        if (!complete) {
            try {
                await reader.cancel();
            }
            catch { /* Reader ownership still ends below. */ }
        }
        reader.releaseLock();
    }
}
export { readBoundedBytes as readStatementBytes };
