package spec

// foundationCorePagePresentationBindings is the closed, compile-time bridge
// between handwritten Foundation pages and presentation metadata. YAML may
// select one of these bindings and set presentation defaults, but it cannot
// provide transport, permission, route, renderer, or action implementations.
var foundationCorePagePresentationBindings = map[string]corePagePresentationBinding{
	"administration.users": corePageBinding(
		"administration.users", "user.list", []string{"/users"},
		[]corePagePresentationFieldBinding{
			coreStringField("username", "账号", "Account", "identifier", true, "user-identity", "", 210),
			coreStringField("name", "姓名", "Name", "plain", false, "text", "input", 0),
			coreStringField("email", "邮箱", "Email", "email", false, "text", "", 0),
			coreStringField("roleName", "角色", "Role", "plain", false, "user-role", "", 150),
			coreStringField("organization", "组织", "Organization", "plain", false, "user-organization", "", 0),
			coreEnumField("status", "状态", "Status", true, "status-tag", "status-filter", 120, administrationStatusValues()),
		},
		[]string{"username", "name", "email", "roleName", "organization", "status"},
		[]string{"name", "status"},
	),
	"administration.roles": corePageBinding(
		"administration.roles", "role.list", []string{"/role"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "名称", "Name", "plain", true, "text", "input", 0),
			coreStringField("classification", "分类", "Classification", "plain", false, "role-classification", "", 0),
			coreEnumField("status", "状态", "Status", true, "status-tag", "status-filter", 120, administrationStatusValues()),
			coreStringField("remark", "备注", "Remark", "plain", false, "text", "", 0),
		},
		[]string{"name", "classification", "status", "remark"},
		[]string{"name", "status"},
	),
	"administration.menus": corePageBinding(
		"administration.menus", "menu.list", []string{"/menu"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "名称", "Name", "plain", true, "menu-label", "input", 240),
			coreStringField("path", "路径", "Path", "plain", false, "code", "", 0),
			coreStringField("type", "类型", "Type", "plain", true, "menu-type", "", 125),
			coreStringField("permission", "权限标识", "Permission", "identifier", false, "text", "", 0),
			coreEnumField("status", "状态", "Status", true, "status-tag", "status-filter", 120, administrationStatusValues()),
		},
		[]string{"name", "path", "type", "permission", "status"},
		[]string{"name", "status"},
	),
	"administration.departments": corePageBinding(
		"administration.departments", "department.list", []string{"/departments"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "名称", "Name", "plain", true, "text", "input", 240),
			coreStringField("code", "编码", "Code", "identifier", false, "code", "", 160),
			coreStringField("leaderID", "负责人", "Leader", "identifier", false, "department-leader", "", 150),
			coreStringField("contact", "联系方式", "Contact", "plain", false, "department-contact", "", 0),
			coreEnumField("status", "状态", "Status", true, "status-tag", "status-filter", 120, administrationStatusValues()),
			coreIntegerField("sort", "排序", "Sort", true, "number", "", 90),
		},
		[]string{"name", "code", "leaderID", "contact", "status", "sort"},
		[]string{"name", "status"},
	),
	"administration.posts": corePageBinding(
		"administration.posts", "post.list", []string{"/posts"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "名称", "Name", "plain", true, "text", "input", 240),
			coreStringField("code", "编码", "Code", "identifier", false, "code", "", 160),
			coreStringField("dataScope", "数据范围", "Data scope", "plain", true, "post-data-scope", "", 0),
			coreEnumField("status", "状态", "Status", true, "status-tag", "status-filter", 120, administrationStatusValues()),
			coreIntegerField("sort", "排序", "Sort", true, "number", "", 90),
		},
		[]string{"name", "code", "dataScope", "status", "sort"},
		[]string{"name", "status"},
	),
	"operations.tasks": corePageBinding(
		"operations.tasks", "task.list", []string{"/task"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "任务名称", "Task name", "plain", true, "text", "input", 0),
			coreEnumField("provider", "执行器", "Provider", true, "task-provider", "", 120, taskProviderValues()),
			coreStringField("spec", "调度规则", "Schedule", "plain", true, "code", "", 180),
			coreEnumField("status", "状态", "Status", true, "task-status", "status-filter", 110, enabledDisabledValues()),
			coreDateTimeField("checkedAt", "最近运行", "Last run", false, "date-time", "", 190),
			coreStringField("remark", "备注", "Remark", "plain", false, "text", "", 0),
		},
		[]string{"name", "provider", "spec", "status", "checkedAt", "remark"},
		[]string{"name", "status"},
	),
	"operations.notices": corePageBinding(
		"operations.notices", "notice.list", []string{"/notice"},
		[]corePagePresentationFieldBinding{
			coreStringField("title", "标题", "Title", "plain", true, "notice-title", "input", 0),
			coreEnumField("type", "类型", "Type", true, "notice-type", "select", 130, noticeTypeValues()),
			coreEnumField("status", "状态", "Status", false, "notice-status", "select", 130, noticeStatusValues()),
			coreStringField("description", "描述", "Description", "plain", false, "text", "", 0),
			coreDateTimeField("createdAt", "发送时间", "Sent at", true, "date-time", "", 190),
		},
		[]string{"title", "type", "status", "description", "createdAt"},
		[]string{"title", "type", "status"},
	),
	"language.languages": corePageBinding(
		"language.languages", "language.list", []string{"/language"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "语言名称", "Language name", "plain", true, "language-name", "input", 0),
			coreEnumField("status", "状态", "Status", true, "language-status", "status-filter", 110, enabledDisabledValues()),
			coreStringField("remark", "备注", "Remark", "plain", false, "text", "", 0),
			coreDateTimeField("updatedAt", "更新时间", "Updated at", true, "date-time", "", 190),
		},
		[]string{"name", "status", "remark", "updatedAt"},
		[]string{"name", "status"},
	),
	"option.options": corePageBinding(
		"option.options", "option.list", []string{"/option"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "选项名称", "Option name", "plain", true, "option-name", "input", 0),
			coreStringField("displayName", "显示名称", "Display name", "plain", false, "text", "", 0),
			coreStringField("category", "分类", "Category", "identifier", true, "code", "input", 140),
			coreEnumField("status", "状态", "Status", true, "option-status", "status-filter", 110, enabledDisabledValues()),
			coreIntegerField("version", "版本", "Version", true, "number", "", 90),
			coreDateTimeField("updatedAt", "更新时间", "Updated at", true, "date-time", "", 190),
		},
		[]string{"name", "displayName", "category", "status", "version", "updatedAt"},
		[]string{"name", "category", "status"},
	),
	"operations.system-configs": corePageBinding(
		"operations.system-configs", "system-config.list", []string{"/system-config"},
		[]corePagePresentationFieldBinding{
			coreStringField("name", "配置名称", "Configuration name", "plain", true, "system-config-name", "", 0),
			coreEnumField("ext", "格式", "Format", true, "system-config-format", "", 100, systemConfigFormatValues()),
			coreStringField("remark", "备注", "Remark", "plain", false, "text", "", 0),
			coreDateTimeField("updatedAt", "更新时间", "Updated at", true, "date-time", "", 190),
		},
		[]string{"name", "ext", "remark", "updatedAt"},
		[]string{},
	),
	"session.online-sessions": corePageBinding(
		"session.online-sessions", "online-session.list", []string{"/security/online-sessions"},
		[]corePagePresentationFieldBinding{
			coreStringField("username", "用户名", "Username", "identifier", true, "online-session-user", "input", 0),
			coreStringField("ip", "IP 地址", "IP address", "plain", false, "text", "input", 0),
			coreStringField("userAgent", "设备", "Device", "plain", false, "online-session-device", "", 0),
			coreDateTimeField("lastSeenAt", "最后活动时间", "Last seen at", true, "date-time", "", 0),
			coreEnumField("status", "状态", "Status", true, "online-session-status", "status-filter", 0, onlineSessionStatusValues()),
		},
		[]string{"username", "ip", "userAgent", "lastSeenAt", "status"},
		[]string{"username", "ip", "status"},
	),
	"operations.login-logs": corePageBinding(
		"operations.login-logs", "log.login", []string{"/log"},
		[]corePagePresentationFieldBinding{
			coreStringField("username", "用户名", "Username", "identifier", false, "text", "input", 160),
			coreStringField("ip", "IP 地址", "IP address", "plain", true, "text", "", 150),
			coreStringField("location", "位置", "Location", "plain", false, "text", "", 0),
			coreEnumField("status", "状态", "Status", false, "status-tag", "", 110, administrationStatusValues()),
			coreStringField("message", "消息", "Message", "plain", false, "operational-message", "", 0),
			coreDateTimeField("loginAt", "登录时间", "Login at", true, "date-time", "", 190),
		},
		[]string{"username", "ip", "location", "status", "message", "loginAt"},
		[]string{"username"},
	),
	"operations.audit-logs": corePageBinding(
		"operations.audit-logs", "log.audit", []string{"/log"},
		[]corePagePresentationFieldBinding{
			coreStringField("username", "用户名", "Username", "identifier", false, "text", "input", 150),
			coreEnumField("type", "类型", "Type", true, "audit-log-type", "select", 120, auditLogTypeValues()),
			coreStringField("action", "操作", "Action", "plain", true, "text", "", 150),
			coreStringField("message", "消息", "Message", "plain", false, "operational-message", "", 0),
			coreStringField("resource", "资源", "Resource", "plain", false, "text", "", 0),
			coreStringField("path", "路径", "Path", "plain", false, "code", "", 0),
			coreEnumField("status", "状态", "Status", false, "status-tag", "", 110, administrationStatusValues()),
			coreIntegerField("duration", "耗时", "Duration", true, "duration", "", 110),
			coreDateTimeField("createdAt", "创建时间", "Created at", true, "date-time", "", 190),
		},
		[]string{"username", "type", "action", "message", "resource", "path", "status", "duration", "createdAt"},
		[]string{"username", "type"},
	),
	"operations.runtime-logs": corePageBinding(
		"operations.runtime-logs", "log.runtime", []string{"/log", "/log/runtime"},
		[]corePagePresentationFieldBinding{
			coreDateTimeField("timestamp", "时间", "Timestamp", true, "date-time", "", 210),
			coreEnumField("level", "级别", "Level", false, "runtime-log-level", "select", 110, runtimeLogLevelValues()),
			coreStringField("message", "消息", "Message", "plain", false, "text", "", 0),
			coreStringField("keyword", "关键词", "Keyword", "plain", false, "", "input", 0),
			coreDateTimeField("timeRange", "时间范围", "Time range", false, "", "date-time-range", 0),
		},
		[]string{"timestamp", "level", "message"},
		[]string{"level", "keyword", "timeRange"},
	),
}

func corePageBinding(
	id string,
	pageKey string,
	requiredPermissions []string,
	fields []corePagePresentationFieldBinding,
	listFields []string,
	searchFields []string,
) corePagePresentationBinding {
	return corePagePresentationBinding{
		ID:                  id,
		PageKey:             pageKey,
		DataSourceID:        pageKey,
		RequiredPermissions: requiredPermissions,
		PageSizeOptions:     []int{20, 50, 100},
		MaxPageSize:         100,
		MaxSortFields:       0,
		Fields:              fields,
		ListFields:          listFields,
		SearchFields:        searchFields,
	}
}

func coreStringField(
	id, zhCN, enUS, format string,
	required bool,
	listComponent, searchComponent string,
	listWidth int,
) corePagePresentationFieldBinding {
	return corePagePresentationFieldBinding{
		ID: id, Label: corePresentationText(zhCN, enUS), ValueType: "string", Format: format,
		Required: required, Nullable: !required, Searchable: searchComponent != "",
		ListComponent: listComponent, SearchComponent: searchComponent, ListWidth: listWidth,
	}
}

func coreIntegerField(
	id, zhCN, enUS string,
	required bool,
	listComponent, searchComponent string,
	listWidth int,
) corePagePresentationFieldBinding {
	return corePagePresentationFieldBinding{
		ID: id, Label: corePresentationText(zhCN, enUS), ValueType: "integer", Format: "plain",
		Required: required, Nullable: !required, Filterable: searchComponent != "",
		ListComponent: listComponent, SearchComponent: searchComponent, ListWidth: listWidth,
	}
}

func coreDateTimeField(
	id, zhCN, enUS string,
	required bool,
	listComponent, searchComponent string,
	listWidth int,
) corePagePresentationFieldBinding {
	return corePagePresentationFieldBinding{
		ID: id, Label: corePresentationText(zhCN, enUS), ValueType: "date-time", Format: "date-time",
		Required: required, Nullable: !required, Filterable: searchComponent != "",
		ListComponent: listComponent, SearchComponent: searchComponent, ListWidth: listWidth,
	}
}

func coreEnumField(
	id, zhCN, enUS string,
	required bool,
	listComponent, searchComponent string,
	listWidth int,
	values []NormalizedPresentationEnumValue,
) corePagePresentationFieldBinding {
	return corePagePresentationFieldBinding{
		ID: id, Label: corePresentationText(zhCN, enUS), ValueType: "enum", Format: "plain",
		Required: required, Nullable: !required, Filterable: searchComponent != "",
		ListComponent: listComponent, SearchComponent: searchComponent, ListWidth: listWidth,
		EnumValues: values,
	}
}

func corePresentationText(zhCN, enUS string) PresentationLocalizedText {
	return PresentationLocalizedText{ZhCN: zhCN, EnUS: enUS}
}

func coreEnumValue(value, zhCN, enUS, color string) NormalizedPresentationEnumValue {
	return NormalizedPresentationEnumValue{
		Value: value, Label: corePresentationText(zhCN, enUS), Color: color,
	}
}

func enabledDisabledValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("disabled", "禁用", "Disabled", "red"),
		coreEnumValue("enabled", "启用", "Enabled", "green"),
	}
}

func administrationStatusValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("disabled", "禁用", "Disabled", "red"),
		coreEnumValue("enabled", "启用", "Enabled", "green"),
		coreEnumValue("locked", "锁定", "Locked", "orange"),
	}
}

func menuTypeValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("api", "接口", "API", "purple"),
		coreEnumValue("component", "组件", "Component", "blue"),
		coreEnumValue("directory", "目录", "Directory", "default"),
		coreEnumValue("menu", "菜单", "Menu", "green"),
	}
}

func taskProviderValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("default", "默认", "Default", "default"),
		coreEnumValue("func", "函数", "Function", "purple"),
		coreEnumValue("k8s", "Kubernetes", "Kubernetes", "blue"),
	}
}

func noticeTypeValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("event", "事件", "Event", "gold"),
		coreEnumValue("mail", "邮件", "Mail", "default"),
		coreEnumValue("message", "消息", "Message", "blue"),
		coreEnumValue("notification", "通知", "Notification", "default"),
	}
}

func noticeStatusValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("doing", "进行中", "Doing", "blue"),
		coreEnumValue("processing", "处理中", "Processing", "blue"),
		coreEnumValue("todo", "待处理", "Todo", "gold"),
		coreEnumValue("urgent", "紧急", "Urgent", "red"),
	}
}

func systemConfigFormatValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("json", "JSON", "JSON", "blue"),
		coreEnumValue("yaml", "YAML", "YAML", "green"),
		coreEnumValue("yml", "YML", "YML", "green"),
	}
}

func onlineSessionStatusValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("active", "在线", "Active", "green"),
		coreEnumValue("expired", "已过期", "Expired", "default"),
		coreEnumValue("revoked", "已撤销", "Revoked", "red"),
	}
}

func auditLogTypeValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("config", "配置", "Config", "default"),
		coreEnumValue("create", "创建", "Create", "green"),
		coreEnumValue("delete", "删除", "Delete", "red"),
		coreEnumValue("export", "导出", "Export", "blue"),
		coreEnumValue("import", "导入", "Import", "blue"),
		coreEnumValue("login", "登录", "Login", "green"),
		coreEnumValue("logout", "退出", "Logout", "default"),
		coreEnumValue("security", "安全", "Security", "orange"),
		coreEnumValue("update", "更新", "Update", "blue"),
	}
}

func runtimeLogLevelValues() []NormalizedPresentationEnumValue {
	return []NormalizedPresentationEnumValue{
		coreEnumValue("debug", "调试", "Debug", "default"),
		coreEnumValue("error", "错误", "Error", "red"),
		coreEnumValue("fatal", "致命", "Fatal", "red"),
		coreEnumValue("info", "信息", "Info", "blue"),
		coreEnumValue("trace", "跟踪", "Trace", "default"),
		coreEnumValue("warn", "警告", "Warn", "gold"),
	}
}
