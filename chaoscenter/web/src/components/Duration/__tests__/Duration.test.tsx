import '@testing-library/jest-dom/extend-expect';
import React from 'react';
import { render, screen } from '@testing-library/react';
import { TestWrapper } from 'utils/testUtils';
import Duration from '../Duration';

describe('Duration', () => {
  test('renders a fixed duration with default prefix (getString returns key)', () => {
    const start = 1000;
    const end = 6000; // 5s delta
    render(
      <TestWrapper>
        <Duration startTime={start} endTime={end} />
      </TestWrapper>
    );
    // default prefix text comes from getString('duration') which echoes the key
    expect(screen.getByText(/duration/)).toBeInTheDocument();
    expect(screen.getByText(/5s/)).toBeInTheDocument();
  });

  test('honours custom durationText override', () => {
    render(
      <TestWrapper>
        <Duration startTime={1000} endTime={2000} durationText={'Elapsed: '} />
      </TestWrapper>
    );
    expect(screen.getByText(/Elapsed:/)).toBeInTheDocument();
  });

  test('renders 0s for sub-second when showZeroSecondsResult set', () => {
    render(
      <TestWrapper>
        <Duration startTime={1000} endTime={1300} durationText={''} showZeroSecondsResult />
      </TestWrapper>
    );
    expect(screen.getByText(/0s/)).toBeInTheDocument();
  });
});
