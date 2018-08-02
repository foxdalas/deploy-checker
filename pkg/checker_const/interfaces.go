package checker

import "github.com/sirupsen/logrus"

type Checker interface {
	Version() string
	Log() *logrus.Entry
}
