DROP TABLE if exists users;
DROP TABLE if exists profiles;
DROP TABLE if exists groups;
DROP TABLE if exists vault_entries;
DROP TYPE if exists entry_type;

DROP if exists trigger set_timestamp on users;
DROP if exists trigger set_timestamp on profiles;
DROP if exists trigger set_timestamp on groups;
DROP if exists trigger set_timestamp on vault_entries;