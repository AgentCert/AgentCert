import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { render, screen } from '@testing-library/react';
import { TestWrapper } from 'utils/testUtils';
import CodeBlock from '../CodeBlock';

describe('CodeBlock', () => {
  test('renders the provided text', () => {
    render(
      <TestWrapper>
        <CodeBlock text="kubectl get pods" />
      </TestWrapper>
    );
    expect(screen.getByText('kubectl get pods')).toBeInTheDocument();
  });

  test('does not render copy button when disabled/omitted', () => {
    render(
      <TestWrapper>
        <CodeBlock text="echo hi" />
      </TestWrapper>
    );
    expect(screen.queryByTestId('copy-button')).not.toBeInTheDocument();
  });

  test('renders copy button when enabled', () => {
    render(
      <TestWrapper>
        <CodeBlock text="echo hi" isCopyButtonEnabled />
      </TestWrapper>
    );
    expect(screen.getByTestId('copy-button')).toBeInTheDocument();
  });
});
