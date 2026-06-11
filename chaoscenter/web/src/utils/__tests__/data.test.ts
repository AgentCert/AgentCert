import { cleanApolloResponse, sanitize, destructure } from '../data';

describe('cleanApolloResponse', () => {
  test('removes __typename fields recursively', () => {
    const input = {
      __typename: 'Root',
      name: 'exp',
      child: { __typename: 'Child', value: 1 },
      list: [{ __typename: 'Item', id: 'a' }]
    };
    const out = cleanApolloResponse(input) as any;
    expect(out.__typename).toBeUndefined();
    expect(out.child.__typename).toBeUndefined();
    expect(out.child.value).toBe(1);
    expect(out.list[0].__typename).toBeUndefined();
    expect(out.list[0].id).toBe('a');
    expect(out.name).toBe('exp');
  });
});

describe('sanitize', () => {
  test('removes null/undefined/empty by default', () => {
    const out = sanitize({
      a: 'x',
      b: null,
      c: undefined,
      d: '',
      e: [],
      f: {},
      g: { keep: 'v' }
    });
    expect(out).toEqual({ a: 'x', g: { keep: 'v' } });
  });

  test('honours config flags to keep empties', () => {
    const out = sanitize(
      { a: '', b: [], c: {} },
      { removeEmptyString: false, removeEmptyArray: false, removeEmptyObject: false }
    );
    expect(out).toEqual({ a: '', b: [], c: {} });
  });

  test('recurses into nested non-empty objects and arrays', () => {
    const out = sanitize({ nested: { a: null, b: 'keep' }, arr: ['x'] });
    expect(out).toEqual({ nested: { b: 'keep' }, arr: ['x'] });
  });
});

describe('destructure', () => {
  test('flattens nested object keeping primitives', () => {
    const out = destructure({
      a: 'str',
      b: true,
      c: 5,
      nested: { d: 'deep', e: 10 }
    });
    expect(out).toEqual({ a: 'str', b: true, c: 5, d: 'deep', e: 10 });
  });

  test('skips NaN numbers', () => {
    const out = destructure({ valid: 1, bad: NaN });
    expect(out).toEqual({ valid: 1 });
  });
});
