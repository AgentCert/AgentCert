import React from 'react';
import cx from 'classnames';
import { Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { ModelTestResult } from '@api/entities';
import { useStrings } from '@strings';
import css from './ModelConfiguration.module.scss';

interface ModelTestResultViewProps {
  testing: boolean;
  result?: ModelTestResult;
}

// The outcome of sending a model one test prompt.
export default function ModelTestResultView({ testing, result }: ModelTestResultViewProps): React.ReactElement | null {
  const { getString } = useStrings();
  if (testing) {
    return (
      <Text icon="loading" color={Color.GREY_600}>
        {getString('modelTesting')}
      </Text>
    );
  }
  if (!result) {
    return null;
  }
  if (result.ok) {
    return (
      <Text
        className={cx(css.testResult, css.testPassed)}
        icon="tick"
        color={Color.GREEN_700}
        data-testid="model-test-passed"
      >
        {getString('modelTestPassed', { latency: result.latencyMs ?? 0 })}
      </Text>
    );
  }
  return (
    <Layout.Vertical className={cx(css.testResult, css.testFailed)} spacing="xsmall" data-testid="model-test-failed">
      <Text icon="warning-sign" color={Color.RED_700} font={{ variation: FontVariation.BODY2 }}>
        {getString('modelTestFailed')}
      </Text>
      <Text color={Color.GREY_800} font={{ size: 'small' }}>
        {result.error}
      </Text>
    </Layout.Vertical>
  );
}
