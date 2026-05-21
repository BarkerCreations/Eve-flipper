package db

import "time"

// SetAgentCooldown suppresses order_id from the agent queue for the given duration.
func (d *DB) SetAgentCooldown(orderID int64, typeID int32, duration time.Duration) {
	d.SetAgentCooldownUntil(orderID, typeID, time.Now().Add(duration))
}

// SetAgentCooldownUntil suppresses order_id until the given absolute time.
func (d *DB) SetAgentCooldownUntil(orderID int64, typeID int32, until time.Time) {
	d.sql.Exec(
		`INSERT INTO agent_cooldowns (order_id, type_id, suppressed_until) VALUES (?, ?, ?)
		 ON CONFLICT(order_id) DO UPDATE SET suppressed_until = excluded.suppressed_until`,
		orderID, typeID, until.Unix(),
	)
}

// IsAgentCooldownActive returns true if order_id is still within its suppression window.
func (d *DB) IsAgentCooldownActive(orderID int64, typeID int32) bool {
	var until int64
	err := d.sql.QueryRow(
		`SELECT suppressed_until FROM agent_cooldowns WHERE order_id = ?`,
		orderID,
	).Scan(&until)
	if err != nil {
		return false
	}
	return time.Now().Unix() < until
}

// GetLatestStationScanID returns the scan history ID of the most recent station scan.
func (d *DB) GetLatestStationScanID() (int64, bool) {
	var id int64
	err := d.sql.QueryRow(
		`SELECT id FROM scan_history WHERE tab = 'station' ORDER BY id DESC LIMIT 1`,
	).Scan(&id)
	if err != nil {
		return 0, false
	}
	return id, true
}
