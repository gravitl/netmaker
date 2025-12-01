USE netmaker;

CREATE TABLE IF NOT EXISTS flows (
    -- Identity
    flow_id            String,
    host_id            String,
    network_id         String,

    -- Flow metadata
    protocol           UInt16,
    src_port           UInt16,
    dst_port           UInt16,
    icmp_type          UInt8,
    icmp_code          UInt8,
    direction          Enum8('ingress'=1, 'egress'=2),

    -- Participants
    src_ip             String,
    src_type           Enum8('host'=1,'user'=2,'extclient'=3,'external'=4),
    src_entity_id      String,

    dst_ip             String,
    dst_type           Enum8('host'=1,'user'=2,'extclient'=3,'external'=4),
    dst_entity_id      String,

    -- Timestamps
    start_ts           DateTime64(3),
    end_ts             DateTime64(3),

    -- Metrics
    bytes_sent         UInt64,
    bytes_recv         UInt64,
    packets_sent       UInt64,
    packets_recv       UInt64,

    -- Conntrack status bitmask
    status             UInt32,

    -- Logical version / event time (for merging)
    version            UInt64
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(start_ts)
ORDER BY (network_id, host_id, flow_id, start_ts);
