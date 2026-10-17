import * as semver from 'semver';

/**
 * Reports whether Grafana serves its core PostgreSQL plugin instead of this build.
 * Grafana 13.2.0 removed the core plugin (grafana/grafana#128782).
 */
export function servesCorePlugin(grafanaVersion: string): boolean {
  return semver.lt(grafanaVersion, '13.2.0');
}
