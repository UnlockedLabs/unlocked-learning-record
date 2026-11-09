# Learning Record migrations

Goose `.sql` migrations, embedded in the binary and run by `app migrate`. They
run after core's, and their versions are tracked in
`public.goose_db_version_learningrecord`, the ledger core used while Learning
Record was its module.

`00001_move_into_learning_record_schema.sql` is byte for byte the file core
shipped, so a site where core already ran it records it as applied and
`app migrate` does nothing there. A test pins its checksum.

Never edit a migration that has been merged; add a new one.

Create new tables as `learning_record.<name>`. Migrations run as core's
database user, so an unqualified table lands in `public`, where the
`learning_record` role can't use it.
