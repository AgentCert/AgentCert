import { ExperimentRunStatus } from '@api/entities';
import {
  trimString,
  capitalize,
  replaceHyphen,
  replaceSpace,
  normalize,
  toTitleCase,
  phaseToUI,
  toSentenceCase
} from '../strings';

describe('trimString', () => {
  test('truncates with ellipsis when longer than length', () => {
    expect(trimString('hello world', 5)).toBe('hello...');
  });
  test('returns as-is when within length', () => {
    expect(trimString('hi', 5)).toBe('hi');
  });
});

describe('capitalize', () => {
  test('uppercases first character', () => {
    expect(capitalize('chaos')).toBe('Chaos');
  });
  test('empty string stays empty', () => {
    expect(capitalize('')).toBe('');
  });
});

describe('replaceHyphen', () => {
  test('replaces first hyphen with space', () => {
    expect(replaceHyphen('a-b-c')).toBe('a b-c');
  });
});

describe('replaceSpace', () => {
  test('collapses multiple spaces into one', () => {
    expect(replaceSpace('a    b   c')).toBe('a b c');
  });
});

describe('normalize', () => {
  test('capitalizes first word and appends remaining hyphen words', () => {
    expect(normalize('pod-delete-fault')).toBe('Pod delete fault ');
  });
  test('single word', () => {
    expect(normalize('pod')).toBe('Pod ');
  });
});

describe('toTitleCase', () => {
  test('title-cases tokens split by separator', () => {
    expect(toTitleCase({ text: 'pod_delete', separator: '_' })).toBe('Pod Delete');
  });
  test('noCasing lowercases everything', () => {
    expect(toTitleCase({ text: 'POD_DELETE', separator: '_', noCasing: true })).toBe('pod delete');
  });
});

describe('phaseToUI', () => {
  test('NA -> N/A', () => {
    expect(phaseToUI('NA')).toBe('N/A');
  });
  test('completed-with-probe-failure maps to COMPLETED', () => {
    expect(phaseToUI(ExperimentRunStatus.COMPLETED_WITH_PROBE_FAILURE)).toBe(
      ExperimentRunStatus.COMPLETED.toUpperCase()
    );
  });
  test('completed-with-error maps to COMPLETED', () => {
    expect(phaseToUI(ExperimentRunStatus.COMPLETED_WITH_ERROR)).toBe(ExperimentRunStatus.COMPLETED.toUpperCase());
  });
  test('other phases uppercased with underscores replaced', () => {
    expect(phaseToUI('some_phase')).toBe('SOME PHASE');
  });
});

describe('toSentenceCase', () => {
  test('uppercases only first char, keeps rest', () => {
    expect(toSentenceCase('hELLO')).toBe('HELLO');
  });
});
