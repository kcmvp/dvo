package sqlx

import (
	"database/sql"
	"strings"
	"sync"
)

var (
	once      sync.Once
	defaultDS *sql.DB
)

const (
	DSKey       = "datasource"
	UserKey     = "${user}"
	PasswordKey = "${password}"
	HostKey     = "${host}"
	DefaultDS   = "DefaultDS"
)

type dataSource struct {
	DB       string   `mapstructure:"db"`
	Driver   string   `mapstructure:"driver"`
	User     string   `mapstructure:"user"`
	Password string   `mapstructure:"password"`
	Host     string   `mapstructure:"host"`
	URL      string   `mapstructure:"url"`
	Scripts  []string `mapstructure:"scripts"`
}

func (ds dataSource) DSN() string {
	dsn := strings.ReplaceAll(ds.URL, UserKey, ds.User)
	dsn = strings.ReplaceAll(dsn, PasswordKey, ds.Password)
	return strings.ReplaceAll(dsn, HostKey, ds.Host)
}
