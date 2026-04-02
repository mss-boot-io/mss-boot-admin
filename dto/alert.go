package dto

type AlertRuleSearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	Name     string `form:"name" json:"name"`
	Metric   string `form:"metric" json:"metric"`
	Status   string `form:"status" json:"status"`
}

func (e *AlertRuleSearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	return int64(e.Current)
}

func (e *AlertRuleSearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	return int64(e.PageSize)
}

type AlertHistorySearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	RuleID   string `form:"ruleId" json:"ruleId"`
	Status   string `form:"status" json:"status"`
}

func (e *AlertHistorySearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	return int64(e.Current)
}

func (e *AlertHistorySearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	return int64(e.PageSize)
}