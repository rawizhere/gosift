package telegram

import (
	"fmt"
	"log/slog"
)

type slogAdapter struct {
	log *slog.Logger
}

func (a slogAdapter) Debugf(format string, args ...any) { a.log.Debug(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Infof(format string, args ...any)  { a.log.Info(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Warnf(format string, args ...any)  { a.log.Warn(fmt.Sprintf(format, args...)) }
func (a slogAdapter) Errorf(format string, args ...any) { a.log.Error(fmt.Sprintf(format, args...)) }
