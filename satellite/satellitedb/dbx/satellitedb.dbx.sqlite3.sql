-- AUTOGENERATED BY gopkg.in/spacemonkeygo/dbx.v1
-- DO NOT EDIT
CREATE TABLE accounting_raws (
	id INTEGER NOT NULL,
	node_id TEXT NOT NULL,
	interval_end_time TIMESTAMP NOT NULL,
	data_total INTEGER NOT NULL,
	data_type INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE accounting_rollups (
	id INTEGER NOT NULL,
	node_id TEXT NOT NULL,
	start_time TIMESTAMP NOT NULL,
	interval INTEGER NOT NULL,
	data_type INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE accounting_timestamps (
	name TEXT NOT NULL,
	value TIMESTAMP NOT NULL,
	PRIMARY KEY ( name )
);
CREATE TABLE bwagreements (
	signature BLOB NOT NULL,
	data BLOB NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( signature )
);
CREATE TABLE injuredsegments (
	info BLOB NOT NULL,
	PRIMARY KEY ( info )
);
CREATE TABLE irreparabledbs (
	segmentpath BLOB NOT NULL,
	segmentdetail BLOB NOT NULL,
	pieces_lost_count INTEGER NOT NULL,
	seg_damaged_unix_sec INTEGER NOT NULL,
	repair_attempt_count INTEGER NOT NULL,
	PRIMARY KEY ( segmentpath )
);
CREATE TABLE nodes (
	id BLOB NOT NULL,
	audit_success_count INTEGER NOT NULL,
	total_audit_count INTEGER NOT NULL,
	audit_success_ratio REAL NOT NULL,
	uptime_success_count INTEGER NOT NULL,
	total_uptime_count INTEGER NOT NULL,
	uptime_ratio REAL NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE overlay_cache_nodes (
	key BLOB NOT NULL,
	value BLOB NOT NULL,
	PRIMARY KEY ( key ),
	UNIQUE ( key )
);
