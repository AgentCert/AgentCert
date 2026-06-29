import {
  getFormattedTime,
  getBriefDuration,
  getDetailedTime,
  getTimeDiff,
  timeDifferenceForDate,
  getDurationBetweenTwoDates,
  handleTimestampAmbiguity,
  zeroFiftyNineDDOptions,
  oneFiftyNineDDOptions,
  oneTwelveDDOptions,
  amPmOptions
} from '../dates';

// Fixed reference timestamps (UTC-independent assertions where possible)
const T0 = new Date('2023-01-01T00:00:00Z').getTime();

describe('getFormattedTime', () => {
  test('returns "-" for falsy', () => {
    expect(getFormattedTime(0)).toBe('-');
  });
  test('formats a real timestamp', () => {
    expect(getFormattedTime(T0)).toMatch(/\d{1,2}\/\d{2}\/\d{4}, \d{2}:\d{2}:\d{2}/);
  });
});

describe('getBriefDuration', () => {
  test('returns "--" when both NaN', () => {
    expect(getBriefDuration(NaN, NaN)).toBe('--');
  });
  test('returns "--" when start falsy', () => {
    expect(getBriefDuration(0, T0)).toBe('--');
  });
  test('returns "--" when end is NaN (falsy guard short-circuits)', () => {
    // NaN is falsy via !endTime so the "Running" branch is unreachable here
    expect(getBriefDuration(T0, NaN)).toBe('--');
  });
  test('computes hms duration', () => {
    const start = T0;
    const end = T0 + (2 * 3600 + 3 * 60 + 4) * 1000; // 2h 3m 4s
    expect(getBriefDuration(start, end)).toBe('2h 3m 4s');
  });
  test('includes days', () => {
    const end = T0 + 25 * 3600 * 1000; // 1d 1h
    expect(getBriefDuration(T0, end)).toBe('1d 1h ');
  });
});

describe('getDetailedTime', () => {
  test('matches detailed format', () => {
    expect(getDetailedTime(T0)).toMatch(/\d{1,2} \w{3} \d{4}, \d{2}:\d{2}/);
  });
});

describe('getTimeDiff', () => {
  test('returns "-" when start NaN', () => {
    expect(getTimeDiff(NaN, T0)).toBe('-');
  });
  test('returns "-" when no endTime', () => {
    expect(getTimeDiff(T0)).toBe('-');
  });
  test('returns range string', () => {
    expect(getTimeDiff(T0, T0 + 1000)).toContain(' - ');
  });
});

describe('timeDifferenceForDate', () => {
  test('returns "-" for falsy date', () => {
    expect(timeDifferenceForDate(0)).toBe('-');
  });
  test('"Just now" for very recent', () => {
    expect(timeDifferenceForDate(Date.now() - 1000)).toBe('Just now');
  });
  test('minutes ago', () => {
    expect(timeDifferenceForDate(Date.now() - 5 * 60 * 1000)).toMatch(/mins ago/);
  });
  test('hours ago', () => {
    expect(timeDifferenceForDate(Date.now() - 5 * 60 * 60 * 1000)).toMatch(/hours ago/);
  });
  test('days ago', () => {
    expect(timeDifferenceForDate(Date.now() - 5 * 24 * 60 * 60 * 1000)).toMatch(/days ago/);
  });
});

describe('getDurationBetweenTwoDates', () => {
  test('returns "-" when both undefined', () => {
    expect(getDurationBetweenTwoDates(undefined, undefined)).toBe('-');
  });
  test('open-ended when only start present', () => {
    expect(getDurationBetweenTwoDates(T0, undefined)).toContain('- --');
  });
  test('same day uses single date with time range', () => {
    const out = getDurationBetweenTwoDates(T0, T0 + 3600 * 1000);
    expect(out).toContain(' - ');
  });
  test('different days uses two dates', () => {
    const out = getDurationBetweenTwoDates(T0, T0 + 5 * 24 * 3600 * 1000);
    expect(out).toContain(' - ');
  });
});

describe('handleTimestampAmbiguity', () => {
  test('seconds (10 chars) converted to ms', () => {
    expect(handleTimestampAmbiguity('1700000000')).toBe('1700000000000');
  });
  test('already ms left untouched', () => {
    expect(handleTimestampAmbiguity('1700000000000')).toBe('1700000000000');
  });
});

describe('option arrays', () => {
  test('zeroFiftyNineDDOptions has 60 entries, padded', () => {
    expect(zeroFiftyNineDDOptions).toHaveLength(60);
    expect(zeroFiftyNineDDOptions[0]).toEqual({ label: '00', value: '0' });
    expect(zeroFiftyNineDDOptions[59]).toEqual({ label: '59', value: '59' });
  });
  test('oneFiftyNineDDOptions drops the zero entry', () => {
    expect(oneFiftyNineDDOptions).toHaveLength(59);
    expect(oneFiftyNineDDOptions[0]).toEqual({ label: '01', value: '1' });
  });
  test('oneTwelveDDOptions covers 1..12', () => {
    expect(oneTwelveDDOptions).toHaveLength(12);
    expect(oneTwelveDDOptions[0]).toEqual({ label: '01', value: '1' });
    expect(oneTwelveDDOptions[11]).toEqual({ label: '12', value: '12' });
  });
  test('amPmOptions', () => {
    expect(amPmOptions).toEqual([
      { label: 'AM', value: 'AM' },
      { label: 'PM', value: 'PM' }
    ]);
  });
});
