package config

type Notification struct {
	Email    EmailConfig    `yaml:"email" json:"email"`
	DingTalk DingTalkConfig `yaml:"dingtalk" json:"dingtalk"`
	WeChat   WeChatConfig   `yaml:"wechat" json:"wechat"`
}

type EmailConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	From     string `yaml:"from" json:"from"`
}

type DingTalkConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Webhook string `yaml:"webhook" json:"webhook"`
	Secret  string `yaml:"secret" json:"secret"`
}

type WeChatConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Webhook string `yaml:"webhook" json:"webhook"`
}