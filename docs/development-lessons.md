# Development Lessons

## React Dynamic Form Rows

- Do not derive React row keys from editable field values such as mapping target, schema path, or field name.
- Use stable row IDs that survive user edits. If a row is serialized to schema/config and then parsed back, preserve draft row IDs during active editing or only rehydrate draft rows when switching records.
- Otherwise, each keystroke can change the key, remount the input, and cause focus loss.
- For rows that temporarily allow empty names, keep a local draft state and only commit valid rows to persisted config.
