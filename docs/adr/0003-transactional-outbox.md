# ADR 0003: Transactional outbox

The transfer transaction writes durable event obligations. A worker publishes them afterwards with repeat-safe handling.
