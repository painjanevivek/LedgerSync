import { accountIDs, tenantIDs, subjectID, validateImageSet, fixedBudgets } from './definition.mjs';
export function capacitySeedSQL() {
    const statements = ['\\set ON_ERROR_STOP on', 'BEGIN;'];
    for (const [i, tenant] of tenantIDs.entries()) {
        statements.push(`INSERT INTO tenants(id,external_reference) VALUES('${tenant}','synthetic-http-capacity-${i}');`);
        statements.push(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES('${tenant}','${subjectID}','operator'),('${tenant}','${subjectID}','finance');`);
        statements.push(`INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor) VALUES('${tenant}','INR',1,1000000000,5000000000,5000000000,10000000000);`);
    }
    for (const [i, id] of accountIDs.entries()) {
        const tenant = tenantIDs[i < 5 ? 0 : 1], minor = i === 4 || i === 7 ? 0 : 1000000;
        statements.push(`INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference) VALUES('${id}','${tenant}','INR','active','Synthetic capacity ${i}','operating','HTTP-CAPACITY-${i}');`);
        statements.push(`INSERT INTO account_opening_balances(account_id,opening_ledger_minor) VALUES('${id}',${minor});`);
        statements.push(`INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version) VALUES('${id}',${minor},${minor},0);`);
        statements.push(`INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) VALUES('${tenant}','${id}','${subjectID}','debit');`);
        statements.push(`INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES('${tenant}','${id}','${subjectID}');`);
    }
    statements.push('COMMIT;');
    return statements.join('\n') + '\n';
}
export function composeCapacityCase({ base, selected, images, project, ports, seedPath, routerPath, collectorPath, sourceSHA }) {
    validateImageSet(images);
    if (!/^ledgersync-capacity-\d{14}-[a-f0-9]{8}$/.test(project) || !/^[a-f0-9]{64}$/.test(sourceSHA) || !Array.isArray(ports) || ports.length !== 2 || ports[0] === ports[1] || ports.some(p => !Number.isInteger(p) || p < 1024 || p > 65535))
        throw Error('Exact capacity project, source and BFF ports required');
    const services = {}, budgets = {}, labels = {
        'ledgersync.capacity.owner': project,
        'ledgersync.capacity.source': sourceSHA
    };
    function add(name, template, image, budget) {
        const service = structuredClone(base.services[template]);
        if (!service)
            throw Error('Required base service is absent');
        for (const key of ['build', 'profiles', 'container_name', 'extends', 'deploy', 'volumes', 'ports'])
            delete service[key];
        Object.assign(service, {
            image,
            restart: 'no',
            read_only: true,
            cap_drop: ['ALL'],
            security_opt: ['no-new-privileges:true'],
            pids_limit: 256,
            cpus: budget.cpu_millicores / 1000,
            mem_limit: `${budget.memory_mib}m`,
            labels: {
                ...labels
            },
            networks: ['private'],
            tmpfs: ['/tmp:rw,noexec,nosuid,size=16m']
        });
        services[name] = service;
        budgets[name] = budget;
        return service;
    }
    const pg = add('postgres', 'postgres', images.postgres, fixedBudgets.postgres);
    pg.tmpfs.push('/var/lib/postgresql/data:rw,size=512m,uid=70,gid=70,mode=0700', '/var/run/postgresql:rw,noexec,nosuid,size=4m');
    const redis = add('redis', 'redis', images.redis, fixedBudgets.redis);
    redis.command = ['redis-server', '--appendonly', 'no', '--save', ''];
    redis.tmpfs.push('/data:rw,size=128m,uid=999,gid=1000,mode=0700');
    const migrate = add('migrate', 'migrate', images.migrate, {
        cpu_millicores: 1000,
        memory_mib: 512
    });
    migrate.tmpfs.push('/var/lib/postgresql/data:rw,size=1m,uid=70,gid=70,mode=0700');
    const seed = add('seed', 'demo-seed', images.migrate, {
        cpu_millicores: 1000,
        memory_mib: 256
    });
    seed.entrypoint = ['psql'];
    seed.command = ['-v', 'ON_ERROR_STOP=1', '-h', 'postgres', '-U', 'ledgersync', '-d', 'ledgersync', '-f', '/seed/http-capacity.sql'];
    seed.depends_on = {
        migrate: {
            condition: 'service_completed_successfully'
        }
    };
    seed.volumes = [{
            type: 'bind',
            source: seedPath,
            target: '/seed/http-capacity.sql',
            read_only: true
        }];
    seed.tmpfs.push('/var/lib/postgresql/data:rw,size=1m,uid=70,gid=70,mode=0700');
    for (let i = 0; i < selected.api_instances; i++) {
        const api = add(i === 0 ? 'api' : 'api-peer', 'api', images.api, {
            cpu_millicores: selected.api_budget.cpu_millicores_per_replica,
            memory_mib: selected.api_budget.memory_mib_per_replica
        });
        delete api.environment.LEDGERSYNC_RECOVERY_EVIDENCE_ROOT;
        api.environment.LEDGERSYNC_DATABASE_MAX_OPEN_CONNS = String(selected.allocation.id === 'two-api-fixed-total' ? 2 : 4);
        api.environment.LEDGERSYNC_DEVELOPMENT_SUBJECT_ID = subjectID;
        api.environment.LEDGERSYNC_DEVELOPMENT_TENANT_ID = tenantIDs[0];
        api.depends_on = {
            seed: {
                condition: 'service_completed_successfully'
            },
            redis: {
                condition: 'service_healthy'
            }
        };
    }
    const worker = add('outbox-worker', 'outbox-worker', images['outbox-worker'], fixedBudgets.worker);
    worker.environment.LEDGERSYNC_DATABASE_MAX_OPEN_CONNS = '2';
    worker.depends_on = {
        seed: {
            condition: 'service_completed_successfully'
        },
        redis: {
            condition: 'service_healthy'
        }
    };
    for (const [name, service] of Object.entries(services))
        if (['api', 'api-peer', 'outbox-worker'].includes(name))
            Object.assign(service.environment, {
                LEDGERSYNC_TELEMETRY_ENABLED: 'true',
                LEDGERSYNC_TELEMETRY_SERVICE_NAME: `ledgersync-capacity-${name}`,
                OTEL_EXPORTER_OTLP_ENDPOINT: 'http://telemetry:4318',
                OTEL_METRIC_EXPORT_INTERVAL: '5000'
            });
    const collector = add('telemetry', 'web', images.telemetry, fixedBudgets.telemetry);
    collector.entrypoint = ['/otelcol-contrib'];
    collector.command = ['--config=/capacity/collector.json'];
    collector.environment = {};
    collector.user = '10001:10001';
    collector.volumes = [{
            type: 'bind',
            source: collectorPath,
            target: '/capacity/collector.json',
            read_only: true
        }];
    delete collector.healthcheck;
    delete collector.depends_on;
    const router = add('api-router', 'web', images.web, fixedBudgets.router);
    router.environment = {
        NODE_ENV: 'production',
        LEDGERSYNC_CAPACITY_BACKENDS: selected.api_instances === 1 ? 'http://api:8080' : 'http://api:8080,http://api-peer:8080'
    };
    router.entrypoint = ['node', '/capacity/router.mjs'];
    router.command = [];
    delete router.healthcheck;
    router.depends_on = Object.fromEntries(Array.from({
        length: selected.api_instances
    }, (_, i) => [i === 0 ? 'api' : 'api-peer', {
            condition: 'service_healthy'
        }]));
    router.volumes = [{
            type: 'bind',
            source: routerPath,
            target: '/capacity/router.mjs',
            read_only: true
        }];
    for (const i of [0, 1]) {
        const web = add(`web-${i}`, 'web', images.web, {
            cpu_millicores: fixedBudgets.web.cpu_millicores / 2,
            memory_mib: fixedBudgets.web.memory_mib / 2
        });
        Object.assign(web.environment, {
            LEDGERSYNC_PRIVATE_API_URL: 'http://api-router:8080',
            LEDGERSYNC_PUBLIC_ORIGIN: `http://127.0.0.1:${ports[i]}`,
            LEDGERSYNC_LOCAL_SUBJECT_ID: subjectID,
            LEDGERSYNC_LOCAL_TENANT_ID: tenantIDs[i]
        });
        web.ports = [{
                target: 3000,
                published: String(ports[i]),
                host_ip: '127.0.0.1',
                protocol: 'tcp'
            }];
        web.networks = ['edge', 'private'];
        web.depends_on = {
            'api-router': {
                condition: 'service_started'
            }
        };
        web.healthcheck = {
            test: ['CMD', 'node', '-e', `require('node:http').get('http://127.0.0.1:3000/',{headers:{host:'127.0.0.1:${ports[i]}'}},r=>{r.resume();process.exit(r.statusCode===200?0:1)}).on('error',()=>process.exit(1))`],
            interval: '3s',
            timeout: '3s',
            retries: 40
        };
    }
    return {
        configuration: {
            name: project,
            services,
            networks: {
                private: {
                    internal: true,
                    labels
                },
                edge: {
                    labels
                }
            },
            volumes: {}
        },
        budgets
    };
}
export function collectorConfiguration() {
    return {
        receivers: {
            otlp: {
                protocols: {
                    http: {
                        endpoint: '0.0.0.0:4318'
                    }
                }
            }
        },
        processors: {
            memory_limiter: {
                check_interval: '1s',
                limit_mib: 256,
                spike_limit_mib: 32
            },
            'transform/privacy': {
                error_mode: 'propagate',
                trace_statements: [{
                        context: 'resource',
                        statements: ['keep_keys(attributes, ["service.name", "service.version"])']
                    }, {
                        context: 'span',
                        statements: ['keep_keys(attributes, ["operation", "outcome"])', 'set(status.message, "")']
                    }]
            },
            'filter/privacy': {
                error_mode: 'propagate',
                traces: {
                    spanevent: ['true']
                }
            },
            batch: {
                timeout: '1s',
                send_batch_size: 256
            }
        },
        exporters: {
            prometheus: {
                endpoint: '0.0.0.0:8889',
                resource_to_telemetry_conversion: {
                    enabled: true
                }
            },
            debug: {
                verbosity: 'basic'
            }
        },
        service: {
            pipelines: {
                metrics: {
                    receivers: ['otlp'],
                    processors: ['memory_limiter', 'batch'],
                    exporters: ['prometheus']
                },
                traces: {
                    receivers: ['otlp'],
                    processors: ['memory_limiter', 'transform/privacy', 'filter/privacy', 'batch'],
                    exporters: ['debug']
                }
            }
        }
    };
}
