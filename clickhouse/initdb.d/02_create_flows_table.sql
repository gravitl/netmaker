CREATE TABLE IF NOT EXISTS flows (
    -- Identity
    flow_id            String,
    host_id            String,
    host_name          String,
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
    src_type           Enum8('node'=1,'user'=2,'extclient'=3,'egress_route'=4,'external'=5),
    src_entity_id      String,
    src_entity_name    String,

    dst_ip             String,
    dst_type           Enum8('node'=1,'user'=2,'extclient'=3,'egress_route'=4,'external'=5),
    dst_entity_id      String,
    dst_entity_name    String,

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
    version            DateTime64(3)
)
ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMMDD(version)
ORDER BY (network_id, host_id, flow_id, version);
