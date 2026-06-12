package enum

type AccountType int

// 定义账户类型枚举 0 资金账户  1 现货账户 2 合约账户 3 借贷账户  4 理财账户
const (
	FundAccount AccountType = iota
	SpotAccount
	ContractAccount
	LendingAccount
	FinancialAccount
)

// ToString
//
//	@Description: 枚举转字符串
//	@receiver a
//	@return string
func (a AccountType) ToString() string {
	switch a {
	case FundAccount:
		return "资金账户"
	case SpotAccount:
		return "现货账户"
	case ContractAccount:
		return "合约账户"
	case LendingAccount:
		return "借贷账户"
	case FinancialAccount:
		return "理财账户"
	default:
		return "未知账户类型"
	}
}
