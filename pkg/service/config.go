package service

import (
	"github.com/moov-io/base/config"
	"github.com/moov-io/base/log"
	railmsgsql "github.com/moov-io/rail-msg-sql"
)

func LoadConfig(logger log.Logger) (*Config, error) {
	configService := config.NewService(logger)

	global := &GlobalConfig{}
	if err := configService.LoadFromFS(global, railmsgsql.ConfigDefaults); err != nil {
		return nil, err
	}

	cfg := &global.RailMsgSql

	return cfg, nil
}
