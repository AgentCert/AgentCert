import { getBreakdownValues, getSelectedTab, DaysOfWeek } from '../cron';

describe('getBreakdownValues', () => {
  test('parses a simple every-minute style expression', () => {
    // min hour dayOfMonth month dayOfWeek
    const out = getBreakdownValues('5 10 * * *');
    expect(out.minutes).toBe('5');
    expect(out.hours).toBe('10');
    expect(out.amPm).toBe('AM'); // hour 10 <= 12
    expect(out.dayOfMonth).toBe('*');
    expect(out.month).toBe('*');
    expect(out.startMonth).toBe('*');
    expect(out.dayOfWeek).toEqual([]);
  });

  test('handles step values in minutes/hours/month', () => {
    const out = getBreakdownValues('0/15 2/3 * 1/2 MON,TUE');
    expect(out.minutes).toBe('15');
    expect(out.hours).toBe('3');
    expect(out.month).toBe('2');
    expect(out.startMonth).toBe('1');
    expect(out.dayOfWeek).toEqual(['MON', 'TUE']);
  });

  test('PM when hour > 12', () => {
    expect(getBreakdownValues('0 14 * * *').amPm).toBe('PM');
  });
});

describe('getSelectedTab', () => {
  test('Minutes', () => {
    expect(getSelectedTab('5 * * * *')).toBe('Minutes');
  });
  test('Hourly (step in hour)', () => {
    expect(getSelectedTab('0 0/2 * * *')).toBe('Hourly');
  });
  test('Daily (fixed hour)', () => {
    expect(getSelectedTab('0 10 * * *')).toBe('Daily');
  });
  test('Weekly', () => {
    expect(getSelectedTab('0 10 * * MON')).toBe('Weekly');
  });
  test('Monthly (step in month)', () => {
    expect(getSelectedTab('0 10 5 1/2 *')).toBe('Monthly');
  });
  test('Yearly (fixed month)', () => {
    expect(getSelectedTab('0 10 5 6 *')).toBe('Yearly');
  });
  test('fallback to Minutes for all-star', () => {
    expect(getSelectedTab('* * * * *')).toBe('Minutes');
  });
});

describe('DaysOfWeek enum', () => {
  test('has 7 days', () => {
    expect(Object.keys(DaysOfWeek)).toHaveLength(7);
    expect(DaysOfWeek.MON).toBe('MON');
  });
});
