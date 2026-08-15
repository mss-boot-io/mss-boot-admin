package dto

type LoginLogSearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	UserID   string `form:"userID" json:"userID" binding:"omitempty,max=64"`
	Username string `form:"username" json:"username" binding:"omitempty,max=255"`
	IP       string `form:"ip" json:"ip" binding:"omitempty,max=50"`
	Status   string `form:"status" json:"status" binding:"omitempty,oneof=enabled disabled locked"`
}

func (e *LoginLogSearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	if e.Current > 10_000 {
		return 10_000
	}
	return int64(e.Current)
}

func (e *LoginLogSearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	if e.PageSize > 100 {
		return 100
	}
	return int64(e.PageSize)
}

type AuditLogSearch struct {
	Current  int    `form:"current" json:"current"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	UserID   string `form:"userID" json:"userID" binding:"omitempty,max=64"`
	Username string `form:"username" json:"username" binding:"omitempty,max=255"`
	Type     string `form:"type" json:"type" binding:"omitempty,oneof=login logout create update delete export import config security"`
	Action   string `form:"action" json:"action" binding:"omitempty,max=255"`
	Resource string `form:"resource" json:"resource" binding:"omitempty,max=255"`
	IP       string `form:"ip" json:"ip" binding:"omitempty,max=50"`
	Status   string `form:"status" json:"status" binding:"omitempty,oneof=enabled disabled locked"`
}

func (e *AuditLogSearch) GetPage() int64 {
	if e.Current <= 0 {
		return 1
	}
	if e.Current > 10_000 {
		return 10_000
	}
	return int64(e.Current)
}

func (e *AuditLogSearch) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 20
	}
	if e.PageSize > 100 {
		return 100
	}
	return int64(e.PageSize)
}
