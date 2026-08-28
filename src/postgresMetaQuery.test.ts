import { getSchema, showDatabases, showSchemas, showTables } from './postgresMetaQuery';

describe('postgredsMetaQuery.getSchema', () => {
  it('should handle table-names with single quote', () => {
    // testing multi-line with single-quote, double-quote, backtick
    const tableName = `'a''bcd'efg'h'  "a""b" ` + '`x``y`z' + `\n a'b''c`;
    const escapedName = `''a''''bcd''efg''h''  "a""b" ` + '`x``y`z' + `\n a''b''''c`;

    const schemaQuery = getSchema(tableName);

    expect(schemaQuery.includes(escapedName)).toBeTruthy();
    expect(schemaQuery.includes(tableName)).toBeFalsy();
  });
});

describe('postgresMetaQuery.showDatabases', () => {
  it('should return a query selecting non-template databases', () => {
    const sql = showDatabases();
    expect(sql).toContain('pg_database');
    expect(sql).toContain('datistemplate = false');
    expect(sql).toContain('ORDER BY datname');
  });
});

describe('postgresMetaQuery.showSchemas', () => {
  it('should return a query that excludes system schemas', () => {
    const sql = showSchemas();
    expect(sql).toContain('information_schema.schemata');
    expect(sql).toContain("'information_schema'");
    expect(sql).toContain("'pg_catalog'");
    expect(sql).toContain("'pg_toast'");
    expect(sql).toContain("'_timescaledb_cache'");
    expect(sql).toContain('ORDER BY schema_name');
  });
});

describe('postgresMetaQuery.showTables', () => {
  it('should return tables across all non-system schemas when no schema is provided', () => {
    const sql = showTables();
    expect(sql).toContain('information_schema.tables');
    expect(sql).toContain('quote_ident(table_name)');
    expect(sql).toContain('quote_ident(table_schema)');
    expect(sql).not.toContain("quote_ident(table_schema) = quote_ident('");
  });

  it('should filter by schema when a schema is provided', () => {
    const sql = showTables('public');
    expect(sql).toContain("quote_ident(table_schema) = quote_ident('public')");
    expect(sql).toContain('information_schema.tables');
  });

  it('should escape single quotes in schema name', () => {
    const sql = showTables("my'schema");
    expect(sql).toContain("quote_ident(table_schema) = quote_ident('my''schema')");
    expect(sql).not.toContain("my'schema");
  });
});
