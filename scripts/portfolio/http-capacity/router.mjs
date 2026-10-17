import http from 'node:http';
import { pathToFileURL } from 'node:url';
const hop = new Set(['connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding', 'upgrade']);
export function createAPIRouter(backends) {
    if (!Array.isArray(backends) || backends.length < 1 || backends.length > 2 || backends.some(s => {
        const u = new URL(s);
        return u.protocol !== 'http:' || !['api', 'api-peer', '127.0.0.1'].includes(u.hostname) || u.username || u.password || u.pathname !== '/' || u.search || u.hash;
    }))
        throw Error('One or two owned private API backends required');
    let cursor = 0;
    return http.createServer((incoming, outgoing) => {
        if (!incoming.url?.startsWith('/') || incoming.url.startsWith('//')) {
            outgoing.writeHead(400).end();
            return;
        }
        const target = new URL(backends[cursor++ % backends.length]);
        const headers = Object.fromEntries(Object.entries(incoming.headers).filter(([name]) => !hop.has(name)));
        headers.host = target.host;
        let bytes = 0, failed = false;
        const upstream = http.request({
            hostname: target.hostname,
            port: target.port,
            method: incoming.method,
            path: incoming.url,
            headers,
            timeout: 11000
        }, response => {
            outgoing.writeHead(response.statusCode ?? 502, Object.fromEntries(Object.entries(response.headers).filter(([name]) => !hop.has(name))));
            response.pipe(outgoing);
            response.on('error', () => outgoing.destroy());
        });
        const fail = () => {
            if (failed)
                return;
            failed = true;
            upstream.destroy();
            if (!outgoing.headersSent)
                outgoing.writeHead(503, {
                    'Content-Type': 'application/json',
                    'Cache-Control': 'no-store'
                }).end('{"error_code":"capacity_upstream_unavailable"}');
            else
                outgoing.destroy();
        };
        incoming.on('data', chunk => {
            bytes += chunk.length;
            if (bytes > 16384)
                fail();
        });
        incoming.on('aborted', fail);
        upstream.on('error', fail);
        upstream.on('timeout', fail);
        outgoing.on('close', () => upstream.destroy());
        incoming.pipe(upstream);
    });
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
    const values = (process.env.LEDGERSYNC_CAPACITY_BACKENDS ?? '').split(',');
    if (values.some(s => !/^http:\/\/api(?:-peer)?:8080$/.test(s)))
        throw Error('Runtime router refuses external or host backends');
    const server = createAPIRouter(values);
    server.requestTimeout = 15000;
    server.headersTimeout = 10000;
    server.maxConnections = 64;
    server.listen(8080, '0.0.0.0');
    const stop = () => server.close(() => process.exit(0));
    process.once('SIGINT', stop);
    process.once('SIGTERM', stop);
}
