package dto

type LoginLogSearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	UserID   string `form:"userID" json:"userID"`
	Username string `form:"username" json:"username"`
	IP       string `form:"ip" json:"ip"`
	Status   string `form:"status" json:"status"`
}

func (e *LoginLogSearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	return int64(e.Current)
}

func (e *LoginLogSearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	return int64(e.PageSize)
}

type AuditLogSearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	UserID   string `form:"userID" json:"userID"`
	Username string `form:"username" json:"username"`
	Type     string `form:"type" json:"type"`
	Action   string `form:"action" json:"action"`
	Resource string `form:"resource" json:"resource"`
	IP       string `form:"ip" json:"ip"`
	Status   string `form:"status" json:"status"`
}

func (e *AuditLogSearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	return int64(e.Current)
}

func (e *AuditLogSearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	return int64(e.PageSize)
}
