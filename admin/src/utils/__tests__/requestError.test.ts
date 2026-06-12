import { getRequestErrorMessage, getServerErrorMessage, getServerFieldViolations } from '@/utils/requestError';

describe('requestError utils', () => {
  it('prefers server message from BizError info', () => {
    expect(getServerErrorMessage({ info: { message: 'biz error msg' } })).toBe('biz error msg');
    expect(getRequestErrorMessage({ info: { message: 'biz error msg' }, message: 'fallback' })).toBe('biz error msg');
  });

  it('extracts message from response/data', () => {
    expect(getServerErrorMessage({ data: { message: 'data msg' } })).toBe('data msg');
    expect(getServerErrorMessage({ response: { data: { message: 'response msg' } } })).toBe('response msg');
  });

  it('falls back to error.message when no server message', () => {
    expect(getServerErrorMessage({ message: 'axios msg' })).toBeUndefined();
    expect(getRequestErrorMessage({ message: 'axios msg' })).toBe('axios msg');
  });

  it('extracts field violations', () => {
    expect(
      getServerFieldViolations({
        response: {
          data: {
            details: { field_violations: [{ field: 'password', description: 'too weak' }] },
          },
        },
      }),
    ).toEqual([{ field: 'password', description: 'too weak' }]);
  });
});

