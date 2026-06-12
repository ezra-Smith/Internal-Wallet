import { umiRequest } from '@/api/http/umiRequest';

type SecondParameter<T extends (...args: never) => unknown> = Parameters<T>[1];

export type AccountingAccountTypeItem = {
  code: string;
  name: string;
  description?: string;
  normal_side: 'debit' | 'credit' | string;
  asset_codes?: string[];
};

export type AccountingUserBalanceItem = {
  asset_code: string;
  scale: number;
  available: string;
  locked: string;
  available_raw: string;
  locked_raw: string;
};

export type AccountingListAccountTypesResponse = {
  success?: boolean;
  message?: string;
  data?: { items?: AccountingAccountTypeItem[] };
  request_id?: string;
  timestamp?: string;
};

export type AccountingCreateAccountTypeRequest = {
  code: string;
  name: string;
  description?: string;
  normal_side: 'debit' | 'credit' | string;
};

export type AccountingCreateAccountTypeResponse = {
  success?: boolean;
  message?: string;
  data?: { item?: AccountingAccountTypeItem };
  request_id?: string;
  timestamp?: string;
};

export type AccountingUpdateAccountTypeRequest = {
  code?: string;
  name: string;
  description?: string;
  normal_side: 'debit' | 'credit' | string;
};

export type AccountingUpdateAccountTypeResponse = {
  success?: boolean;
  message?: string;
  data?: { item?: AccountingAccountTypeItem };
  request_id?: string;
  timestamp?: string;
};

export type AccountingDeleteAccountTypeResponse = {
  success?: boolean;
  message?: string;
  request_id?: string;
  timestamp?: string;
};

export type AccountingSetAccountTypeAssetsRequest = {
  code?: string;
  asset_codes: string[];
};

export type AccountingSetAccountTypeAssetsResponse = {
  success?: boolean;
  message?: string;
  request_id?: string;
  timestamp?: string;
};

export type AccountingEnsureUserSetupResponse = {
  success?: boolean;
  message?: string;
  request_id?: string;
  timestamp?: string;
};

export type AccountingGetUserBalancesResponse = {
  success?: boolean;
  message?: string;
  data?: { items?: AccountingUserBalanceItem[] };
  request_id?: string;
  timestamp?: string;
};

export type AccountingEnsureSystemAccountsRequest = {
  chain_code?: string;
};

export type AccountingEnsureSystemAccountsResponse = {
  success?: boolean;
  message?: string;
  request_id?: string;
  timestamp?: string;
};

export type AccountingLedgerTxItem = {
  tx_id: string;
  created_at: string;
  op_type: string;
  biz_ref: string;
  idempotency_key: string;
  request_hash: string;
};

export type AccountingListLedgerTxRequest = {
  page?: number;
  page_size?: number;
  tx_id?: string | number;
  op_type?: string;
  biz_ref?: string;
  idempotency_key?: string;
  asset_code?: string;
  user_id?: string | number;
  created_from?: string;
  created_to?: string;
};

export type AccountingListLedgerTxResponse = {
  success?: boolean;
  message?: string;
  items?: AccountingLedgerTxItem[];
  pagination?: { page?: number; page_size?: number; total?: number; total_pages?: number; has_next?: boolean; has_prev?: boolean };
  request_id?: string;
  timestamp?: string;
};

export type AccountingLedgerPostingItem = {
  seq: number;
  asset_code: string;
  scale: number;
  bucket: string;
  account_id: string;
  owner_type: string;
  owner_id: string;
  account_type_code: string;
  account_type_name: string;
  account_type_description: string;
  account_normal_side: string;
  chain_scope: string;
  debit_raw: string;
  credit_raw: string;
  debit_decimal: string;
  credit_decimal: string;
};

export type AccountingGetLedgerTxDetailResponse = {
  success?: boolean;
  message?: string;
  tx?: AccountingLedgerTxItem;
  postings?: AccountingLedgerPostingItem[];
  request_id?: string;
  timestamp?: string;
};

export const adminAccountingListAccountTypes = (
  options?: SecondParameter<typeof umiRequest<AccountingListAccountTypesResponse>>,
) =>
  umiRequest<AccountingListAccountTypesResponse>({ url: `/api/v1/admin/accounting/account-types`, method: 'GET' }, options);

export const adminAccountingCreateAccountType = (
  body: AccountingCreateAccountTypeRequest,
  options?: SecondParameter<typeof umiRequest<AccountingCreateAccountTypeResponse>>,
) =>
  umiRequest<AccountingCreateAccountTypeResponse>(
    { url: `/api/v1/admin/accounting/account-types`, method: 'POST', headers: { 'Content-Type': 'application/json' }, data: body },
    options,
  );

export const adminAccountingUpdateAccountType = (
  code: string,
  body: AccountingUpdateAccountTypeRequest,
  options?: SecondParameter<typeof umiRequest<AccountingUpdateAccountTypeResponse>>,
) =>
  umiRequest<AccountingUpdateAccountTypeResponse>(
    {
      url: `/api/v1/admin/accounting/account-types/${encodeURIComponent(code)}`,
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      data: { ...body, code },
    },
    options,
  );

export const adminAccountingDeleteAccountType = (
  code: string,
  options?: SecondParameter<typeof umiRequest<AccountingDeleteAccountTypeResponse>>,
) =>
  umiRequest<AccountingDeleteAccountTypeResponse>(
    { url: `/api/v1/admin/accounting/account-types/${encodeURIComponent(code)}`, method: 'DELETE' },
    options,
  );

export const adminAccountingSetAccountTypeAssets = (
  code: string,
  body: AccountingSetAccountTypeAssetsRequest,
  options?: SecondParameter<typeof umiRequest<AccountingSetAccountTypeAssetsResponse>>,
) =>
  umiRequest<AccountingSetAccountTypeAssetsResponse>(
    {
      url: `/api/v1/admin/accounting/account-types/${encodeURIComponent(code)}/assets`,
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      data: { ...body, code },
    },
    options,
  );

export const adminAccountingEnsureUserSetup = (
  userId: string | number,
  options?: SecondParameter<typeof umiRequest<AccountingEnsureUserSetupResponse>>,
) =>
  umiRequest<AccountingEnsureUserSetupResponse>(
    {
      url: `/api/v1/admin/accounting/users/${encodeURIComponent(String(userId))}/ensure`,
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      data: {},
    },
    options,
  );

export const adminAccountingGetUserBalances = (
  userId: string | number,
  options?: SecondParameter<typeof umiRequest<AccountingGetUserBalancesResponse>>,
) =>
  umiRequest<AccountingGetUserBalancesResponse>(
    { url: `/api/v1/admin/accounting/users/${encodeURIComponent(String(userId))}/balances`, method: 'GET' },
    options,
  );

export const adminAccountingEnsureSystemAccounts = (
  body: AccountingEnsureSystemAccountsRequest,
  options?: SecondParameter<typeof umiRequest<AccountingEnsureSystemAccountsResponse>>,
) =>
  umiRequest<AccountingEnsureSystemAccountsResponse>(
    { url: `/api/v1/admin/accounting/system/ensure`, method: 'POST', headers: { 'Content-Type': 'application/json' }, data: body },
    options,
  );

export const adminAccountingListLedgerTx = (
  params: AccountingListLedgerTxRequest,
  options?: SecondParameter<typeof umiRequest<AccountingListLedgerTxResponse>>,
) =>
  umiRequest<AccountingListLedgerTxResponse>(
    { url: `/api/v1/admin/accounting/ledger/tx`, method: 'GET', params },
    options,
  );

export const adminAccountingGetLedgerTxDetail = (
  tx_id: string | number,
  options?: SecondParameter<typeof umiRequest<AccountingGetLedgerTxDetailResponse>>,
) =>
  umiRequest<AccountingGetLedgerTxDetailResponse>({ url: `/api/v1/admin/accounting/ledger/tx/${tx_id}`, method: 'GET' }, options);
