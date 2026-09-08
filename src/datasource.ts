import { v4 as uuidv4 } from 'uuid';

import {
  type DataSourceInstanceSettings,
  type ScopedVars,
  type VariableWithMultiSupport,
} from '@grafana/data';
import { type LanguageDefinition } from '@grafana/plugin-ui';
import { type TemplateSrv } from '@grafana/runtime';
import {
  COMMON_FNS,
  type DB,
  type FuncParameter,
  MACRO_FUNCTIONS,
  type SQLQuery,
  type SQLSelectableValue,
  SqlDatasource,
  SQLVariableSupport,
  formatSQL,
} from '@grafana/sql';

import { PostgresQueryModel } from './PostgresQueryModel';
import { getSchema, getTimescaleDBVersion, getVersion, showDatabases, showSchemas, showTables } from './postgresMetaQuery';
import { fetchColumns, fetchTables, getSqlCompletionProvider } from './sqlCompletionProvider';
import { getFieldConfig, toRawSql } from './sqlUtil';
import { type PostgresOptions } from './types';

export class PostgresDatasource extends SqlDatasource {
  sqlLanguageDefinition: LanguageDefinition | undefined = undefined;

  constructor(instanceSettings: DataSourceInstanceSettings<PostgresOptions>) {
    super(instanceSettings);
    this.dialect = 'postgres';
    this.variables = new SQLVariableSupport(this);
  }

  async testDatasource() {
    const database = (this.instanceSettings.jsonData as PostgresOptions).database;
    if (!database) {
      return {
        status: 'error',
        message:
          'You do not currently have a default database configured for this data source. Postgres requires a default database with which to connect. Please configure one in the Connection section above.',
      };
    }
    return super.testDatasource();
  }

  getQueryModel(target?: SQLQuery, templateSrv?: TemplateSrv, scopedVars?: ScopedVars): PostgresQueryModel {
    return new PostgresQueryModel(target, templateSrv, scopedVars);
  }

  applyTemplateVariables(target: SQLQuery, scopedVars: ScopedVars) {
    return {
      refId: target.refId,
      datasource: this.getRef(),
      rawSql: this.templateSrv.replace(target.rawSql, scopedVars, this.interpolateVariable),
      format: target.format,
      ...(target.database && { database: this.templateSrv.replace(target.database, scopedVars) }),
      ...(target.dataset && { dataset: this.templateSrv.replace(target.dataset, scopedVars) }),
      ...(target.table && { table: this.templateSrv.replace(target.table, scopedVars) }),
    };
  }

  interpolateVariable = (value: string | string[] | number, variable: VariableWithMultiSupport) => {
    if (typeof value === 'string') {
      // For single string values, just escape quotes (don't add outer quotes)
      // The quotes are provided by the query template: WHERE x = '$var'
      // We only escape internal single quotes: O'Brien -> O''Brien
      return String(value).replace(/'/g, "''");
    }

    if (typeof value === 'number') {
      return value;
    }

    if (Array.isArray(value)) {
      // For arrays, quote each value individually and join with comma
      // Used in: WHERE x IN ($var) -> WHERE x IN ('val1','val2','val3')
      const quotedValues = value.map((v) => this.getQueryModel().quoteLiteral(v));
      return quotedValues.join(',');
    }

    return value;
  };

  async getVersion(): Promise<string> {
    const value = await this.runSql<{ version: number }>(getVersion());
    const results = value.fields.version?.values;

    if (!results) {
      return '';
    }

    return results[0].toString();
  }

  async getTimescaleDBVersion(): Promise<string | undefined> {
    const value = await this.runSql<{ extversion: string }>(getTimescaleDBVersion());
    const results = value.fields.extversion?.values;

    if (!results) {
      return undefined;
    }

    return results[0];
  }

  async fetchDatabases(): Promise<string[]> {
    const result = await this.runSql<{ datname: string }>(showDatabases(), { refId: 'databases' });
    const databases = result.fields.datname?.values ?? [];
    const variables = (this.templateSrv?.getVariables?.() ?? [])
      .map((v) => `$${v.name}`)
      .filter((v) => !databases.includes(v));
    return [...databases, ...variables];
  }

  async fetchSchemas(database?: string): Promise<string[]> {
    const result = await this.runSql<{ schema_name: string }>(showSchemas(), { refId: 'schemas', database });
    return result.fields.schema_name?.values ?? [];
  }

  async fetchTables(schema?: string, database?: string): Promise<string[]> {
    const tables = await this.runSql<{ table: string[] }>(showTables(schema), { refId: 'tables', database });
    return tables.fields.table?.values.flat() ?? [];
  }

  getSqlLanguageDefinition(db: DB): LanguageDefinition {
    if (this.sqlLanguageDefinition !== undefined) {
      return this.sqlLanguageDefinition;
    }

    const args = {
      getColumns: { current: (query: SQLQuery) => fetchColumns(db, query) },
      getTables: { current: () => fetchTables(db) },
    };
    this.sqlLanguageDefinition = {
      id: 'pgsql',
      completionProvider: getSqlCompletionProvider(args),
      formatter: formatSQL,
    };
    return this.sqlLanguageDefinition;
  }

  async fetchFields(query: SQLQuery): Promise<SQLSelectableValue[]> {
    const { table, database } = query;
    if (table === undefined) {
      return [];
    }
    const sql = getSchema(table);
    const schema = await this.runSql<{ column: string; type: string }>(sql, { refId: `columns-${uuidv4()}`, database });
    const result: SQLSelectableValue[] = [];
    for (let i = 0; i < schema.length; i++) {
      const column = schema.fields.column.values[i];
      const type = schema.fields.type.values[i];
      result.push({ label: column, value: column, type, ...getFieldConfig(type) });
    }
    return result;
  }

  getFunctions = (): ReturnType<DB['functions']> => {
    const columnParam: FuncParameter = {
      name: 'Column',
      required: true,
      options: (query) => this.fetchFields(query),
    };

    return [...MACRO_FUNCTIONS(columnParam), ...COMMON_FNS.map((fn) => ({ ...fn, parameters: [columnParam] }))];
  };

  getDB(): DB {
    if (this.db !== undefined) {
      return this.db;
    }

    const enableMultiDatabase = (this.instanceSettings.jsonData as PostgresOptions).enableMultiDatabase === true;

    const db: DB = {
      init: () => Promise.resolve(true),
      datasets: (database?: string) => this.fetchSchemas(database),
      tables: (schema?: string, database?: string) => this.fetchTables(schema, database),
      getEditorLanguageDefinition: () => this.getSqlLanguageDefinition(this.db),
      fields: async (query: SQLQuery) => {
        if (!query?.table) {
          return [];
        }
        return this.fetchFields(query);
      },
      validateQuery: (query) =>
        Promise.resolve({ isError: false, isValid: true, query, error: '', rawSql: query.rawSql }),
      toRawSql,
      functions: () => this.getFunctions(),
      lookup: async () => {
        const tables = await this.fetchTables();
        return tables.map((t) => ({ name: t, completion: t }));
      },
      labels: new Map([['dataset', 'Schema']]),
    };

    if (enableMultiDatabase) {
      db.databases = () => this.fetchDatabases();
    }

    return db;
  }
}
