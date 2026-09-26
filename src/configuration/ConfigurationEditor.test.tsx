import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { type DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { type PostgresOptions, PostgresTLSMethods, PostgresTLSModes, PostgresTLSNegotiations } from '../types';

import { PostgresConfigEditor } from './ConfigurationEditor';

const NEGOTIATION_LABEL = 'TLS/SSL Negotiation';

function setup(jsonData: Partial<PostgresOptions> = {}) {
  const onOptionsChange = jest.fn();
  const props = {
    options: {
      name: 'postgres',
      url: 'localhost:5432',
      user: 'grafana',
      jsonData: {
        database: 'testdb',
        postgresVersion: 1500,
        tlsConfigurationMethod: PostgresTLSMethods.filePath,
        ...jsonData,
      },
      secureJsonFields: {},
    },
    onOptionsChange,
  } as unknown as DataSourcePluginOptionsEditorProps<PostgresOptions>;

  render(<PostgresConfigEditor {...props} />);
  return { onOptionsChange };
}

describe('PostgresConfigEditor', () => {
  describe('TLS/SSL Negotiation field', () => {
    it('is rendered when TLS is enabled', () => {
      setup({ sslmode: PostgresTLSModes.require });

      expect(screen.getByText(NEGOTIATION_LABEL)).toBeInTheDocument();
    });

    it('is not rendered when TLS is disabled', () => {
      setup({ sslmode: PostgresTLSModes.disable });

      expect(screen.queryByText(NEGOTIATION_LABEL)).not.toBeInTheDocument();
    });

    it('defaults to postgres negotiation when unset', () => {
      setup({ sslmode: PostgresTLSModes.require });

      expect(screen.getByDisplayValue(PostgresTLSNegotiations.postgres)).toBeInTheDocument();
    });

    it('shows the configured negotiation value', () => {
      setup({ sslmode: PostgresTLSModes.require, sslNegotiation: PostgresTLSNegotiations.direct });

      expect(screen.getByDisplayValue(PostgresTLSNegotiations.direct)).toBeInTheDocument();
    });

    it('writes the selected value to sslNegotiation in jsonData', async () => {
      const { onOptionsChange } = setup({ sslmode: PostgresTLSModes.require });

      await userEvent.click(screen.getByDisplayValue(PostgresTLSNegotiations.postgres));
      await userEvent.keyboard('{ArrowDown}{Enter}');

      expect(onOptionsChange).toHaveBeenCalledWith(
        expect.objectContaining({
          jsonData: expect.objectContaining({ sslNegotiation: PostgresTLSNegotiations.direct }),
        })
      );
    });
  });
});
