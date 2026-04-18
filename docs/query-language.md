# PathCollapse Query Language

PathCollapse includes a human-readable DSL for analyst-driven investigation.

## Statement Types

### FIND PATHS

Find lateral movement paths between two identity references.

```
FIND PATHS FROM <ref> TO <ref>
  [WHERE <predicate> [AND <predicate>...]]
  [ORDER BY <field> [DESC|ASC]]
  [LIMIT <n>]
```

**Examples**:
```
FIND PATHS FROM user:alice TO privilege:tier0 WHERE confidence > 0.7 ORDER BY risk DESC LIMIT 10

FIND PATHS FROM service_account:svc-sql TO privilege:tier0

FIND PATHS FROM user:helpdesk1 TO computer:DC01 WHERE exploitability > 0.5 LIMIT 5
```

**Reference kinds** (`<kind>:<value>`):
- `user:<name>` — match user nodes by name
- `group:<name>` — match group nodes by name
- `computer:<name>` — match computer nodes by name
- `service_account:<name>` — match service account nodes
- `privilege:tier0` / `privilege:tier1` / `privilege:tier2` — match by tag

**WHERE predicates** (applied as edge filters):
- `confidence > 0.7`
- `exploitability >= 0.5`
- `detectability < 0.3`
- `blast_radius != 0.0`
- Multiple predicates joined with `AND`

**ORDER BY fields**: `risk`, `confidence`, `exploitability`

---

### FIND BREAKPOINTS

Compute the minimal control changes to collapse the top risk paths.

```
FIND BREAKPOINTS FOR <target> [LIMIT <n>]
```

**Examples**:
```
FIND BREAKPOINTS FOR top_paths LIMIT 5
FIND BREAKPOINTS FOR top_paths LIMIT 20
```

Output: ordered list of recommended changes, each showing:
- Change description
- Number of paths collapsed
- Aggregate risk reduction
- Implementation difficulty

---

### SHOW DRIFT

Report changes between the current graph and a previous snapshot.

```
SHOW DRIFT [SINCE <snapshot-label>]
```

**Examples**:
```
SHOW DRIFT SINCE last_snapshot
SHOW DRIFT
```

For detailed drift analysis use the `diff` subcommand:
```bash
pathcollapse diff old-snapshot.json new-snapshot.json
```

---

### FIND HIGH_RISK_SERVICE_ACCOUNTS

Shorthand to find service accounts with paths to tier-0 assets.

```
FIND HIGH_RISK_SERVICE_ACCOUNTS
```

---

## Grammar (EBNF)

```ebnf
statement   = find_stmt | show_stmt
find_stmt   = "FIND" (paths_stmt | breakpoints_stmt | highRisk_stmt)
paths_stmt  = "PATHS" "FROM" ref "TO" ref [where_clause] [order_clause] [limit_clause]
break_stmt  = "BREAKPOINTS" "FOR" ident [limit_clause]
highRisk    = ident  (* HIGH_RISK_SERVICE_ACCOUNTS etc. *)
show_stmt   = "SHOW" "DRIFT" ["SINCE" ident]

where_clause  = "WHERE" predicate ("AND" predicate)*
predicate     = ident op value
op            = ">" | "<" | ">=" | "<=" | "=" | "!="
value         = number | string | ident
order_clause  = "ORDER" "BY" ident ["DESC" | "ASC"]
limit_clause  = "LIMIT" number

ref = ident [":" ident]
```

---

## Error Messages

| Error | Cause |
|-------|-------|
| `parse query: lex: unexpected character ...` | Invalid character in query |
| `executor: FROM ref ... resolved to no nodes` | Node name not found in graph |
| `parser: expected PATHS or BREAKPOINTS after FIND` | Malformed FIND statement |
