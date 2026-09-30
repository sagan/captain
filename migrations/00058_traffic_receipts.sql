-- +goose Up
CREATE TABLE node_traffic_receipts (
    node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    epoch TEXT NOT NULL,
    seq INTEGER NOT NULL,
    PRIMARY KEY (node_id, epoch)
);
